import Combine
import Foundation

/// Opens the provider's page and returns the `orbit://` URL it redirects to.
/// `ASWebAuthenticationSession` in the app.
@MainActor
public protocol WebAuthenticator {
    func authenticate(url: URL, callbackScheme: String) async throws -> URL
}

/// The user closed the sign-in sheet.
public struct SignInCancelled: Error {
    public init() {}
}

/// State and actions of the sign-in screen.
@MainActor
public final class SignInModel: ObservableObject {
    public enum Provider: String, Sendable {
        case github, google
    }

    /// Which part of the screen an error belongs next to.
    public enum Area: Equatable, Sendable { case server, providers, email }

    @Published public var serverText: String {
        didSet {
            // What was loaded for one server must not be offered for another.
            guard ServerAddress.parse(oldValue) != ServerAddress.parse(serverText) else { return }
            methods = nil
            client = nil
            emailSent = false
            errorMessage = nil
        }
    }
    @Published public var email = ""
    @Published public private(set) var methods: SignInMethods?
    /// What is being waited for, or nil.
    @Published public private(set) var activity: String?
    @Published public private(set) var errorMessage: String?
    @Published public private(set) var errorArea = Area.server
    /// Counts errors, so the same message twice is still announced twice.
    @Published public private(set) var errorCount = 0
    /// Why the user is back on this screen, if there is a reason.
    @Published public private(set) var notice: String?
    @Published public private(set) var emailSent = false
    @Published public private(set) var signedIn: Bool

    private let makeClient: @MainActor (URL) -> APIClient
    private let defaults: UserDefaults
    private var client: APIClient?
    private var connecting: Task<Void, Never>?
    /// Kept until the redirect arrives: only this verifier can use the code, and
    /// only the server it was made for may see either.
    private var pending: (pkce: PKCE, server: URL)?

    private static let serverKey = "orbit.server"
    private static let verifierKey = "orbit.pkce"
    private static let verifierServerKey = "orbit.pkce.server"

    /// `sessionServer` is the server the stored tokens belong to, if any. It wins
    /// over the last typed address (the Keychain outlives deleting the app).
    public init(defaults: UserDefaults = .standard, sessionServer: URL? = nil, makeClient: @escaping @MainActor (URL) -> APIClient) {
        self.defaults = defaults
        self.makeClient = makeClient
        serverText = sessionServer?.absoluteString ?? defaults.string(forKey: Self.serverKey) ?? ""
        signedIn = sessionServer != nil
        // An emailed link may relaunch the app, so the verifier must survive.
        // It is useless without the code sent to the user's inbox.
        if let verifier = defaults.string(forKey: Self.verifierKey),
           let server = defaults.string(forKey: Self.verifierServerKey).flatMap(URL.init(string:)) {
            pending = (PKCE(verifier: verifier), server)
        }
    }

    public var busy: Bool { activity != nil }

    /// Checks the server address and loads which sign-in methods it offers.
    /// Callers that overlap share one attempt.
    public func connect() async {
        if let connecting {
            await connecting.value
            return
        }
        let task = Task { await loadMethods() }
        connecting = task
        await task.value
        connecting = nil
    }

    private func loadMethods() async {
        errorMessage = nil
        methods = nil
        guard let url = ServerAddress.parse(serverText) else {
            fail(Self.badAddress, in: .server)
            return
        }
        activity = "Checking server…"
        defer { activity = nil }
        let candidate = makeClient(url)
        do {
            let offered = try await candidate.methods()
            // The user may have typed another address meanwhile.
            guard ServerAddress.parse(serverText) == url else { return }
            serverText = url.absoluteString
            methods = offered
            client = candidate
            notice = nil
            defaults.set(url.absoluteString, forKey: Self.serverKey)
        } catch APIError.unreachable {
            fail(Self.describe(APIError.unreachable), in: .server)
        } catch {
            // A normal website answers, just not like Orbit.
            fail("That address didn't answer like an Orbit server. Check the address and try again.", in: .server)
        }
    }

    public func validateServer() {
        if !serverText.isEmpty && ServerAddress.parse(serverText) == nil {
            fail(Self.badAddress, in: .server)
        }
    }

    @discardableResult
    public func validateEmail() -> Bool {
        let address = email.trimmingCharacters(in: .whitespacesAndNewlines)
        let valid = address.contains("@") && !address.hasPrefix("@") && !address.hasSuffix("@")
        if !valid {
            fail("Enter an email address, like name@example.com.", in: .email)
        } else if errorArea == .email {
            errorMessage = nil
        }
        return valid
    }

    public func signIn(with provider: Provider, using web: WebAuthenticator) async {
        guard let client, !busy else { return }
        errorMessage = nil
        activity = "Signing in…"
        defer { activity = nil }
        let pkce = PKCE.random()
        remember(pkce, for: client.server)
        var parts = URLComponents(url: client.server.appendingPathComponent("api/auth/\(provider.rawValue)/start"), resolvingAgainstBaseURL: false)
        parts?.queryItems = [
            URLQueryItem(name: "client", value: "ios"),
            URLQueryItem(name: "code_challenge", value: pkce.challenge),
        ]
        guard let url = parts?.url else { return }
        do {
            let callback = try await web.authenticate(url: url, callbackScheme: "orbit")
            await handle(callback: callback)
        } catch is SignInCancelled {
            // Nothing to report: the user chose to close it.
        } catch {
            fail(Self.describe(error), in: .providers)
        }
    }

    public func sendEmailLink() async {
        guard let client, !busy else { return }
        errorMessage = nil
        guard validateEmail() else { return }
        activity = "Sending link…"
        defer { activity = nil }
        let pkce = PKCE.random()
        remember(pkce, for: client.server)
        do {
            try await client.requestEmailLink(email: email.trimmingCharacters(in: .whitespacesAndNewlines), pkce: pkce)
            emailSent = true
        } catch {
            fail(Self.describe(error), in: .email)
        }
    }

    /// The one client for the chosen server. It serialises token refreshes, so
    /// everything must share it.
    public func currentClient() -> APIClient? {
        if client == nil, let url = ServerAddress.parse(serverText) {
            client = makeClient(url)
        }
        return client
    }

    public func signOut() async {
        activity = "Signing out…"
        defer { activity = nil }
        await currentClient()?.signOut()
        forget()
        emailSent = false
        methods = nil
        errorMessage = nil
        signedIn = false
    }

    /// The server no longer accepts this session.
    public func sessionEnded() async {
        await signOut()
        notice = "Your session ended. Sign in again."
    }

    public func useDifferentEmail() {
        emailSent = false
    }

    /// Handles `orbit://auth/callback?code=…` or `?error=…`.
    public func handle(callback: URL) async {
        guard callback.scheme == "orbit", callback.host == "auth", callback.path == "/callback",
              let items = URLComponents(url: callback, resolvingAgainstBaseURL: false)?.queryItems
        else { return }
        // Ignore a callback nobody here asked for: any app or page can open this URL.
        guard let (pkce, server) = pending else { return }
        if let failure = items.first(where: { $0.name == "error" })?.value {
            fail(Self.explain(failure), in: .providers)
            return
        }
        guard let code = items.first(where: { $0.name == "code" })?.value else { return }
        // Never hand one server's code and verifier to another.
        guard ServerAddress.parse(serverText) == server else {
            fail("That sign-in was started for a different server. Start again.", in: .providers)
            return
        }
        guard let client = currentClient() else { return }
        let before = activity
        activity = "Signing in…"
        defer { activity = before }
        do {
            try await client.signIn(code: code, pkce: pkce)
            forget()
            notice = nil
            signedIn = true
        } catch {
            fail(Self.describe(error), in: .providers)
        }
    }

    private func fail(_ message: String, in area: Area) {
        errorArea = area
        errorMessage = message
        errorCount += 1
    }

    private static let badAddress = "Use an HTTPS address with nothing after the host, like orbit.example.com."

    private func remember(_ pkce: PKCE, for server: URL) {
        pending = (pkce, server)
        defaults.set(pkce.verifier, forKey: Self.verifierKey)
        defaults.set(server.absoluteString, forKey: Self.verifierServerKey)
    }

    private func forget() {
        pending = nil
        defaults.removeObject(forKey: Self.verifierKey)
        defaults.removeObject(forKey: Self.verifierServerKey)
    }

    private static func describe(_ error: Error) -> String {
        (error as? LocalizedError)?.errorDescription ?? "Something went wrong. Try again."
    }

    private static func explain(_ code: String) -> String {
        switch code {
        case "access_denied": return "Sign-in was cancelled. Try again when you're ready."
        case "email_unverified": return "That account has no verified email. Verify it there, or use an email link."
        case "account_exists": return "You already have an Orbit account with this email. Sign in the way you did before, or use an email link."
        default: return "The sign-in service didn't respond. Try again."
        }
    }
}
