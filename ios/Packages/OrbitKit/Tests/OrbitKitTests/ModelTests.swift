import XCTest
@testable import OrbitKit

/// Lets a test change what the server answers.
private final class Server: @unchecked Sendable {
    private let lock = NSLock()
    private var overrides: [String: (Int, String)] = [:]

    func set(_ key: String, _ status: Int, _ body: String = "") { lock.withLock { overrides[key] = (status, body) } }

    func answer(_ request: URLRequest) -> (Int, String) {
        let key = "\(request.httpMethod!) \(request.url!.path)"
        if let found = lock.withLock({ overrides[key] }) { return found }
        switch key {
        case "GET /api/sources": return (200, #"{"sources":[{"id":"s1","url":"https://example.com/s.json","name":"One","created_at":"x"}]}"#)
        case "POST /api/sources/preview": return (200, #"{"url":"https://apps.example.com/s.json","name":"Indie","app_count":2,"apps":["A"]}"#)
        case "POST /api/sources": return (201, #"{"id":"s2","url":"u","name":"n","created_at":"x"}"#)
        case "DELETE /api/sources/s1": return (204, "")
        case "POST /api/devices": return (201, #"{"id":"d1","name":"Ada's iPhone"}"#)
        case "POST /api/devices/d1/report": return (204, "")
        default: return (500, "")
        }
    }
}

@MainActor
final class ModelTests: XCTestCase {
    private let server = Server()
    private lazy var transport = StubTransport { [server] in server.answer($0) }
    private lazy var client = APIClient(server: serverURL, store: MemoryStore(Tokens(accessToken: "a", refreshToken: "r")), transport: transport)
    private var ended = 0
    private var clock = Date(timeIntervalSince1970: 1_000_000)
    private var defaults: UserDefaults!

    override func setUp() async throws {
        defaults = UserDefaults(suiteName: name)
        defaults.removePersistentDomain(forName: name)
    }

    private func calls() -> [String] {
        transport.requests.map { "\($0.httpMethod!) \($0.url!.path)" }
    }

    private func browse() -> BrowseModel {
        let client = client
        return BrowseModel(client: { client }, sessionEnded: { self.ended += 1 })
    }

    private func device() -> DeviceModel {
        let client = client
        return DeviceModel(defaults: defaults, now: { self.clock }, client: { client }, registration: {
            DeviceRegistration(name: "iPhone", model: "iPhone16,1", iosVersion: "18.0", clientID: "abc12345")
        })
    }

    func testLoadAndRemoveSources() async {
        let model = browse()
        await model.load()
        XCTAssertEqual(model.sources?.map(\.title), ["One"])
        await model.remove(model.sources![0])
        XCTAssertEqual(model.sources, [])
        XCTAssertTrue(calls().contains("DELETE /api/sources/s1"))
    }

    func testPreviewThenSave() async {
        let model = browse()
        model.address = "http://apps.example.com/s.json"
        await model.check()
        XCTAssertNotNil(model.addError)
        XCTAssertTrue(calls().isEmpty)

        model.address = "https://apps.example.com/s.json"
        await model.check()
        XCTAssertEqual(model.preview?.appCount, 2)
        XCTAssertFalse(calls().contains("POST /api/sources"))
        let saved = await model.save()
        XCTAssertTrue(saved)
        XCTAssertEqual(model.address, "")
        XCTAssertNil(model.preview)
    }

    func testEditingTheAddressDropsThePreview() async {
        let model = browse()
        model.address = "https://apps.example.com/s.json"
        await model.check()
        model.address += "x"
        XCTAssertNil(model.preview)
        let saved = await model.save()
        XCTAssertFalse(saved)
    }

    func testServerErrorsAreShownAndKeptOffTheList() async {
        server.set("GET /api/sources", 500, #"{"error":{"code":"x","message":"Broken.","fix":"Wait."}}"#)
        let model = browse()
        await model.load()
        XCTAssertNil(model.sources)
        XCTAssertEqual(model.errorMessage, "Broken. Wait.")
    }

    func testAnEndedSessionIsHandedOn() async {
        server.set("GET /api/sources", 401)
        server.set("POST /api/auth/token", 400, #"{"error":{"code":"invalid_grant","message":"x"}}"#)
        await browse().load()
        XCTAssertEqual(ended, 1)
    }

    func testRegistersOnceThenChecksInAtMostEveryTenMinutes() async {
        let model = device()
        await model.checkIn()
        XCTAssertEqual(model.state, .listed(name: "Ada's iPhone"))
        await model.checkIn()
        XCTAssertEqual(calls(), ["POST /api/devices"])

        clock.addTimeInterval(601)
        await device().checkIn() // a relaunch
        XCTAssertEqual(calls(), ["POST /api/devices", "POST /api/devices/d1/report"])
        XCTAssertEqual(Set(jsonBody(transport.requests[1]).keys), ["reported_at"])
    }

    func testARemovedDeviceIsNotAddedBackUnasked() async {
        let model = device()
        await model.checkIn()
        server.set("POST /api/devices/d1/report", 404, #"{"error":{"code":"not_found","message":"x"}}"#)
        clock.addTimeInterval(601)
        await model.checkIn()
        XCTAssertEqual(model.state, .removed)
        await device().checkIn()
        XCTAssertEqual(calls().filter { $0 == "POST /api/devices" }.count, 1)
        await model.addAgain()
        XCTAssertEqual(calls().filter { $0 == "POST /api/devices" }.count, 2)
    }

    func testTheDeviceLimitIsExplained() async {
        server.set("POST /api/devices", 403, #"{"error":{"code":"device_limit","message":"At the limit.","fix":"Remove one."}}"#)
        let model = device()
        await model.checkIn()
        XCTAssertEqual(model.state, .failed("At the limit. Remove one."))
    }

    func testSigningOutForgetsTheDevice() async {
        let model = device()
        await model.checkIn()
        model.forget()
        XCTAssertEqual(model.state, .unknown)
        await model.checkIn()
        XCTAssertEqual(calls().filter { $0 == "POST /api/devices" }.count, 2)
    }
}
