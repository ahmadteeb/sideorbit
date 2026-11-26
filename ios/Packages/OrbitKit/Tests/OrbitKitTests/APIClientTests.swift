import XCTest
@testable import OrbitKit

final class APIClientTests: XCTestCase {
    private let profileJSON = #"{"id":"u1","email":"a@example.com","identities":["github"],"display_name":null,"limits":{}}"#

    func testExpiredAccessTokenIsRenewedOnce() async throws {
        let store = MemoryStore(Tokens(accessToken: "access-0", refreshToken: "refresh-0"))
        let profile = profileJSON
        let transport = StubTransport { request in
            if request.url!.path == "/api/auth/token" { return (200, tokensJSON(1)) }
            return request.value(forHTTPHeaderField: "Authorization") == "Bearer access-1" ? (200, profile) : (401, "{}")
        }
        let client = APIClient(server: serverURL, store: store, transport: transport)

        // Many calls at once: reusing a refresh token would end the session.
        try await withThrowingTaskGroup(of: Profile.self) { group in
            for _ in 0..<8 { group.addTask { try await client.profile() } }
            for try await result in group { XCTAssertEqual(result.id, "u1") }
        }
        let refreshes = transport.requests.filter { $0.url!.path == "/api/auth/token" }
        XCTAssertEqual(refreshes.count, 1)
        XCTAssertEqual(jsonBody(refreshes[0])["refresh_token"] as? String, "refresh-0")
        XCTAssertEqual(store.load()?.refreshToken, "refresh-1")
    }

    func testRefusedRefreshSignsOut() async {
        let store = MemoryStore(Tokens(accessToken: "a", refreshToken: "r"))
        let transport = StubTransport { request in
            request.url!.path == "/api/auth/token"
                ? (400, #"{"error":{"code":"invalid_grant","message":"No longer valid."}}"#)
                : (401, "{}")
        }
        let client = APIClient(server: serverURL, store: store, transport: transport)
        do {
            _ = try await client.profile()
            XCTFail("expected an error")
        } catch {
            XCTAssertEqual(error as? APIError, .signedOut)
        }
        XCTAssertNil(store.load())
    }

    func testServerTroubleDuringRefreshKeepsTheSession() async {
        let store = MemoryStore(Tokens(accessToken: "a", refreshToken: "r"))
        let transport = StubTransport { request in
            request.url!.path == "/api/auth/token" ? (502, "bad gateway") : (401, "{}")
        }
        let client = APIClient(server: serverURL, store: store, transport: transport)
        _ = try? await client.profile()
        XCTAssertEqual(store.load()?.refreshToken, "r")
    }

    func testSignOutClearsTokensEvenWhenTheServerFails() async {
        let store = MemoryStore(Tokens(accessToken: "a", refreshToken: "r"))
        let transport = StubTransport { _ in (500, "") }
        await APIClient(server: serverURL, store: store, transport: transport).signOut()
        XCTAssertNil(store.load())
        XCTAssertEqual(jsonBody(transport.requests[0])["refresh_token"] as? String, "r")
    }
}
