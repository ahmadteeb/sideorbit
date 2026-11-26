import Foundation

public enum APIError: Error, Equatable {
    /// The session is over; the user has to sign in again.
    case signedOut
    /// The server answered with an error from docs/API.md.
    case server(status: Int, code: String, message: String, fix: String?)
    case unreachable
}

extension APIError: LocalizedError {
    public var errorDescription: String? {
        switch self {
        case .signedOut: return "You're signed out. Sign in again."
        case .server(_, _, let message, let fix): return [message, fix].compactMap { $0 }.joined(separator: " ")
        case .unreachable: return "Couldn't reach your Orbit server. Check your connection and the server address."
        }
    }
}

/// Performs one request. `URLSession` in the app, a stub in tests.
public protocol Transport: Sendable {
    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse)
}

public struct URLSessionTransport: Transport {
    public enum Redirects: Sendable {
        case refuse
        /// For public downloads only; never with credentials.
        case httpsOnly
    }

    private let redirects: Redirects

    public init(redirects: Redirects = .refuse) {
        self.redirects = redirects
    }

    public func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        // Not URLSession.shared: no cache or cookies for authenticated calls.
        let session = URLSession(configuration: .ephemeral, delegate: RedirectPolicy(redirects), delegateQueue: nil)
        defer { session.finishTasksAndInvalidate() }
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw APIError.unreachable }
        return (data, http)
    }
}

/// Orbit's API never redirects its calls, and following one would replay a token
/// or sign-in code to wherever it points. Public downloads may follow HTTPS.
private final class RedirectPolicy: NSObject, URLSessionTaskDelegate {
    private let allowed: URLSessionTransport.Redirects

    init(_ allowed: URLSessionTransport.Redirects) {
        self.allowed = allowed
    }

    func urlSession(
        _ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
        newRequest request: URLRequest, completionHandler: @escaping (URLRequest?) -> Void
    ) {
        let follow = allowed == .httpsOnly && request.url?.scheme?.lowercased() == "https"
        completionHandler(follow ? request : nil)
    }
}

/// The client for Orbit's API.
///
/// A refresh token works once and reusing it signs the session out, so this is
/// an actor with at most one refresh in flight. Share one instance.
public actor APIClient {
    public let server: URL
    private let store: TokenStore
    private let transport: Transport
    private var refreshing: Task<Tokens, Error>?

    public init(server: URL, store: TokenStore, transport: Transport = URLSessionTransport()) {
        self.server = server
        self.store = store
        self.transport = transport
    }

    public var isSignedIn: Bool { tokens != nil }

    /// This server's tokens only.
    private var tokens: Tokens? {
        guard let session = store.load(), session.server == server else { return nil }
        return session.tokens
    }

    private func save(_ tokens: Tokens) throws {
        try store.save(StoredSession(server: server, tokens: tokens))
    }

    public func methods() async throws -> SignInMethods {
        try decode(await perform(request("GET", "api/auth/methods")))
    }

    public func requestEmailLink(email: String, pkce: PKCE) async throws {
        let body = ["email": email, "client": "ios", "code_challenge": pkce.challenge]
        _ = try await perform(request("POST", "api/auth/email/start", body: body))
    }

    /// Exchanges the one-time code from the sign-in redirect for tokens.
    public func signIn(code: String, pkce: PKCE) async throws {
        let body = ["grant_type": "authorization_code", "code": code, "code_verifier": pkce.verifier]
        let tokens: Tokens = try decode(await perform(request("POST", "api/auth/token", body: body)))
        try save(tokens)
    }

    public func profile() async throws -> Profile {
        try decode(await authorized("GET", "api/me"))
    }

    public func report(_ report: DeviceReport, deviceID: String) async throws {
        _ = try await authorized("POST", "api/devices/\(deviceID)/report", data: report.encoded())
    }

    public func sources() async throws -> [SavedSource] {
        struct Body: Decodable { let sources: [SavedSource] }
        let body: Body = try decode(await authorized("GET", "api/sources"))
        return body.sources
    }

    public func previewSource(_ address: String) async throws -> SourcePreview {
        try decode(await authorized("POST", "api/sources/preview", data: JSONEncoder().encode(["url": address])))
    }

    public func addSource(_ address: String) async throws {
        _ = try await authorized("POST", "api/sources", data: JSONEncoder().encode(["url": address]))
    }

    public func deleteSource(id: String) async throws {
        _ = try await authorized("DELETE", "api/sources/\(id)")
    }

    /// Registering the same `clientID` again returns the same device.
    public func registerDevice(_ registration: DeviceRegistration) async throws -> RegisteredDevice {
        try decode(await authorized("POST", "api/devices", data: JSONEncoder().encode(registration)))
    }

    /// Signs out. The local tokens go even if the server can't be reached.
    public func signOut() async {
        guard let old = tokens else { return }
        // Forget them first, so a refresh finishing meanwhile can't restore them.
        store.clear()
        _ = try? await perform(request("POST", "api/auth/logout", body: ["refresh_token": old.refreshToken]))
    }

    private func authorized(_ method: String, _ path: String, data: Data? = nil) async throws -> Data {
        guard let tokens else { throw APIError.signedOut }
        var call = request(method, path, data: data)
        call.setValue("Bearer \(tokens.accessToken)", forHTTPHeaderField: "Authorization")
        do {
            return try await perform(call)
        } catch APIError.server(status: 401, _, _, _) {
            let renewed = try await renew(after: tokens)
            call.setValue("Bearer \(renewed.accessToken)", forHTTPHeaderField: "Authorization")
            return try await perform(call)
        }
    }

    private func renew(after old: Tokens) async throws -> Tokens {
        if let refreshing { return try await refreshing.value }
        // Someone else already renewed while this request was in flight.
        if let current = tokens, current != old { return current }
        let task = Task { () throws -> Tokens in
            let body = ["grant_type": "refresh_token", "refresh_token": old.refreshToken]
            do {
                let renewed: Tokens = try decode(await perform(request("POST", "api/auth/token", body: body)))
                // Signed out while this was in flight: don't bring the session back.
                guard tokens == old else { throw APIError.signedOut }
                try save(renewed)
                return renewed
            } catch APIError.server(status: 400, code: "invalid_grant", _, _) {
                if tokens == old { store.clear() }
                throw APIError.signedOut
            }
        }
        refreshing = task
        defer { refreshing = nil }
        return try await task.value
    }

    private func request(_ method: String, _ path: String, body: [String: String]? = nil, data: Data? = nil) -> URLRequest {
        var call = URLRequest(url: server.appendingPathComponent(path))
        call.httpMethod = method
        call.timeoutInterval = 20
        if let payload = data ?? body.flatMap({ try? JSONEncoder().encode($0) }) {
            call.httpBody = payload
            call.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        return call
    }

    private func perform(_ call: URLRequest) async throws -> Data {
        let data: Data
        let response: HTTPURLResponse
        do {
            (data, response) = try await transport.send(call)
        } catch {
            throw APIError.unreachable
        }
        guard (200..<300).contains(response.statusCode) else {
            struct Envelope: Decodable {
                struct Detail: Decodable {
                    let code: String
                    let message: String
                    let fix: String?
                }
                let error: Detail
            }
            let detail = try? JSONDecoder().decode(Envelope.self, from: data).error
            throw APIError.server(
                status: response.statusCode, code: detail?.code ?? "error",
                message: detail?.message ?? "The server answered with an error.", fix: detail?.fix)
        }
        return data
    }

    private func decode<T: Decodable>(_ data: Data) throws -> T {
        try JSONDecoder().decode(T.self, from: data)
    }
}
