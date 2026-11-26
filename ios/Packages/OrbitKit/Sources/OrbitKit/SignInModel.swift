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

    @Published public var serverText: String
    @Published public var email = ""
    @Published public private(set) var methods: SignInMethods?
    @Published public private(set) var busy = false
    @Published public private(set) var errorMessage: String?
    @Published public private(set) var emailSent = false
    @Published public private(set) var signedIn: Bool

    private let makeClient: @MainActor (URL) -> APIClient
    private let defaults: UserDefaults
    private var client: APIClient?
    /// Kept until the redirect arrives: only this verifier can use the code.
    private var pending: PKCE?

    private static let serverKey = "orbit.server"
    private static let verifierKey = "orbit.pkce"

    /// `signedIn` is whether session tokens are already stored.
    public init(defaults: UserDefaults = .standard, signedIn: Bool = false, makeClient: @escaping @MainActor (URL) -> APIClient) {
        self.signedIn = signedIn
        self.defaults = defaults
        self.makeClient = makeClient
        serverText = defaults.string(forKey: Self.serverKey) ?? ""
        // An emailed link may relaunch the app, so the verifier must survive.
        // It is useless without the code sent to the user's inbox.
        pending = defaults.string(forKey: Self.verifierKey).map(PKCE.init(verifier:))
    }

    /// Checks the server address and loads which sign-in methods it offers.
    public func connect() async {
        errorMessage = nil
        methods = nil
        guard let url = ServerAddress.parse(serverText) else {
            errorMessage = "Enter your server's address, like orbit.example.com."
            return
        }
        busy = true
        defer { busy = false }
        let candidate = makeClient(url)
        do {
            methods = try await candidate.methods()
            client = candidate
            defaults.set(url.absoluteString, forKey: Self.serverKey)
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    public func signIn(with provider: Provider, using web: WebAuthenticator) async {
        guard let client else { return }
        errorMessage = nil
        busy = true
        defer { busy = false }
        let pkce = PKCE.random()
        remember(pkce)
        let server = await client.server
        var parts = URLComponents(url: server.appendingPathComponent("api/auth/\(provider.rawValue)/start"), resolvingAgainstBaseURL: false)
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
            errorMessage = Self.describe(error)
        }
    }

    public func sendEmailLink() async {
        guard let client else { return }
        let address = email.trimmingCharacters(in: .whitespacesAndNewlines)
        errorMessage = nil
        guard address.contains("@"), !address.hasPrefix("@"), !address.hasSuffix("@") else {
            errorMessage = "Enter an email address, like name@example.com."
            return
        }
        busy = true
        defer { busy = false }
        let pkce = PKCE.random()
        remember(pkce)
        do {
            try await client.requestEmailLink(email: address, pkce: pkce)
            emailSent = true
        } catch {
            errorMessage = Self.describe(error)
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

    /// Also used when the server reports that the session has ended.
    public func signOut() async {
        await currentClient()?.signOut()
        forget()
        emailSent = false
        methods = nil
        signedIn = false
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
        guard let pkce = pending else { return }
        if let failure = items.first(where: { $0.name == "error" })?.value {
            errorMessage = Self.explain(failure)
            return
        }
        guard let code = items.first(where: { $0.name == "code" })?.value else { return }
        if client == nil { await connect() }
        guard let client else { return }
        busy = true
        defer { busy = false }
        do {
            try await client.signIn(code: code, pkce: pkce)
            forget()
            signedIn = true
        } catch {
            errorMessage = Self.describe(error)
        }
    }

    private func remember(_ pkce: PKCE) {
        pending = pkce
        defaults.set(pkce.verifier, forKey: Self.verifierKey)
    }

    private func forget() {
        pending = nil
        defaults.removeObject(forKey: Self.verifierKey)
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
