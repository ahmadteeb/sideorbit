import XCTest
@testable import OrbitKit

@MainActor
private struct FakeWeb: WebAuthenticator {
    let result: Result<URL, Error>
    let opened: (URL) -> Void
    func authenticate(url: URL, callbackScheme: String) async throws -> URL {
        opened(url)
        return try result.get()
    }
}

@MainActor
final class SignInModelTests: XCTestCase {
    private var defaults: UserDefaults!
    private let store = MemoryStore()
    private let transport = StubTransport { request in
        switch request.url!.path {
        case "/api/auth/methods": return (200, #"{"github":true,"google":false,"email":true}"#)
        case "/api/auth/token": return (200, tokensJSON(1))
        default: return (202, "")
        }
    }

    override func setUp() async throws {
        defaults = UserDefaults(suiteName: name)
        defaults.removePersistentDomain(forName: name)
    }

    private func model() -> SignInModel {
        let store = store, transport = transport
        return SignInModel(defaults: defaults) { APIClient(server: $0, store: store, transport: transport) }
    }

    private func connected() async -> SignInModel {
        let model = model()
        model.serverText = "orbit.example.com"
        await model.connect()
        return model
    }

    func testBadServerAddressIsExplained() async {
        let model = model()
        model.serverText = "nonsense"
        await model.connect()
        XCTAssertNil(model.methods)
        XCTAssertEqual(model.errorMessage, "Use an HTTPS address with nothing after the host, like orbit.example.com.")
        XCTAssertEqual(model.errorArea, .server)
        XCTAssertTrue(transport.requests.isEmpty)
    }

    func testChangingTheServerDropsItsOptions() async {
        let model = await connected()
        model.email = "a@example.com"
        await model.sendEmailLink()
        model.serverText = "https://orbit.example.com" // the same server, written differently
        XCTAssertNotNil(model.methods)
        model.serverText = "other.example.com"
        XCTAssertNil(model.methods)
        XCTAssertFalse(model.emailSent)
        let before = transport.requests.count
        await model.sendEmailLink()
        XCTAssertEqual(transport.requests.count, before)
    }

    func testStoredTokensWithoutAServerAreNoSession() async {
        let orphaned = SignInModel(defaults: defaults, signedIn: true) { [store, transport] in
            APIClient(server: $0, store: store, transport: transport)
        }
        XCTAssertFalse(orphaned.signedIn)
        _ = await connected()
        let restored = SignInModel(defaults: defaults, signedIn: true) { [store, transport] in
            APIClient(server: $0, store: store, transport: transport)
        }
        XCTAssertTrue(restored.signedIn)
    }

    func testAnEndedSessionSaysWhy() async {
        let model = await connected()
        await model.sessionEnded()
        XCTAssertEqual(model.notice, "Your session ended. Sign in again.")
        await model.connect()
        XCTAssertNil(model.notice)
    }

    func testConnectLoadsMethodsAndRemembersTheServer() async {
        let model = await connected()
        XCTAssertEqual(model.serverText, "https://orbit.example.com")
        XCTAssertEqual(model.methods, SignInMethods(github: true, google: false, email: true))
        XCTAssertEqual(self.model().serverText, "https://orbit.example.com")
    }

    func testOAuthSignInUsesPKCE() async throws {
        let model = await connected()
        var opened: URL?
        let web = FakeWeb(result: .success(URL(string: "orbit://auth/callback?code=abc")!)) { opened = $0 }
        await model.signIn(with: .github, using: web)

        XCTAssertTrue(model.signedIn)
        XCTAssertEqual(store.load()?.accessToken, "access-1")
        let items = URLComponents(url: try XCTUnwrap(opened), resolvingAgainstBaseURL: false)?.queryItems
        let exchange = jsonBody(try XCTUnwrap(transport.requests.last))
        let verifier = try XCTUnwrap(exchange["code_verifier"] as? String)
        XCTAssertEqual(items?.first { $0.name == "code_challenge" }?.value, PKCE.challenge(for: verifier))
        XCTAssertEqual(exchange["code"] as? String, "abc")
    }

    func testClosingTheSheetIsNotAnError() async {
        let model = await connected()
        await model.signIn(with: .github, using: FakeWeb(result: .failure(SignInCancelled())) { _ in })
        XCTAssertNil(model.errorMessage)
        XCTAssertFalse(model.signedIn)
    }

    func testEmailLinkSurvivesARelaunch() async {
        let first = await connected()
        first.email = "a@example.com"
        await first.sendEmailLink()
        XCTAssertTrue(first.emailSent)

        let relaunched = model()
        await relaunched.handle(callback: URL(string: "orbit://auth/callback?code=xyz")!)
        XCTAssertTrue(relaunched.signedIn)
    }

    func testUnrequestedOrForeignCallbacksAreIgnored() async {
        let model = await connected()
        let before = transport.requests.count
        await model.handle(callback: URL(string: "orbit://auth/callback?code=abc")!) // nothing pending
        model.email = "a@example.com"
        await model.sendEmailLink()
        await model.handle(callback: URL(string: "orbit://app/com.example?code=abc")!)
        await model.handle(callback: URL(string: "https://auth/callback?code=abc")!)
        XCTAssertEqual(transport.requests.count, before + 1) // only the email request
        XCTAssertFalse(model.signedIn)
    }

    func testCallbackErrorsGetPlainWords() async {
        let model = await connected()
        let web = FakeWeb(result: .success(URL(string: "orbit://auth/callback?error=account_exists")!)) { _ in }
        await model.signIn(with: .google, using: web)
        XCTAssertTrue(model.errorMessage?.contains("already have an Orbit account") == true)
    }

    func testSignOutForgetsTheSession() async {
        let model = await connected()
        await model.signIn(with: .github, using: FakeWeb(result: .success(URL(string: "orbit://auth/callback?code=abc")!)) { _ in })
        await model.signOut()
        XCTAssertFalse(model.signedIn)
        XCTAssertNil(store.load())
        XCTAssertNil(model.methods)
    }

    func testInvalidEmailSendsNothing() async {
        let model = await connected()
        let before = transport.requests.count
        model.email = "nope"
        await model.sendEmailLink()
        XCTAssertNotNil(model.errorMessage)
        XCTAssertEqual(transport.requests.count, before)
    }
}
