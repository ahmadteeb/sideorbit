import Foundation
@testable import OrbitKit

final class MemoryStore: TokenStore, @unchecked Sendable {
    private let lock = NSLock()
    private var session: StoredSession?

    init(_ tokens: Tokens? = nil) {
        session = tokens.map { StoredSession(server: serverURL, tokens: $0) }
    }
    var tokens: Tokens? { lock.withLock { session?.tokens } }
    func load() -> StoredSession? { lock.withLock { session } }
    func save(_ session: StoredSession) throws { lock.withLock { self.session = session } }
    func clear() { lock.withLock { session = nil } }
}

/// Answers requests from a closure and records them.
final class StubTransport: Transport, @unchecked Sendable {
    private let lock = NSLock()
    private var log: [URLRequest] = []
    private let answer: @Sendable (URLRequest) -> (Int, String)

    init(_ answer: @escaping @Sendable (URLRequest) -> (Int, String)) { self.answer = answer }

    var requests: [URLRequest] { lock.withLock { log } }

    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        lock.withLock { log.append(request) }
        // Let concurrent callers overlap.
        try await Task.sleep(nanoseconds: 10_000_000)
        let (status, body) = answer(request)
        let response = HTTPURLResponse(url: request.url!, statusCode: status, httpVersion: nil, headerFields: nil)!
        return (Data(body.utf8), response)
    }
}

let serverURL = URL(string: "https://orbit.example.com")!

func tokensJSON(_ n: Int) -> String {
    #"{"access_token":"access-\#(n)","refresh_token":"refresh-\#(n)","expires_in":900}"#
}

func jsonBody(_ request: URLRequest) -> [String: Any] {
    guard let data = request.httpBody, let object = try? JSONSerialization.jsonObject(with: data) else { return [:] }
    return object as? [String: Any] ?? [:]
}
