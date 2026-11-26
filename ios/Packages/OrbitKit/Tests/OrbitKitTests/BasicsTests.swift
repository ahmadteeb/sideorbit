import XCTest
@testable import OrbitKit

final class BasicsTests: XCTestCase {
    func testPKCEMatchesRFC7636() {
        XCTAssertEqual(
            PKCE(verifier: "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk").challenge,
            "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM")
    }

    func testRandomVerifiersAreLongAndDifferent() {
        let a = PKCE.random(), b = PKCE.random()
        XCTAssertEqual(a.verifier.count, 43)
        XCTAssertNotEqual(a.verifier, b.verifier)
    }

    func testServerAddress() {
        let accepted = [
            "orbit.example.com": "https://orbit.example.com",
            " https://Orbit.Example.com/ ": "https://orbit.example.com",
            "orbit.example.com:8443": "https://orbit.example.com:8443",
        ]
        for (input, want) in accepted {
            XCTAssertEqual(ServerAddress.parse(input)?.absoluteString, want, input)
        }
        for input in ["", "orbit", "http://orbit.example.com", "https://u:p@orbit.example.com",
                      "https://orbit.example.com/path", "https://orbit.example.com?x=1", "ftp://orbit.example.com"] {
            XCTAssertNil(ServerAddress.parse(input), input)
        }
    }

    func testAppleIDIsAlwaysMasked() {
        let cases = ["jane.doe@icloud.com": "j***@icloud.com", "+15551234567": "+***", "*x@a.com": "***@a.com", "": "***"]
        for (input, want) in cases {
            XCTAssertEqual(DeviceReport.AppleAccount.mask(input), want, input)
        }
    }

    func testSourceAddressLosesItsSecrets() {
        let app = DeviceReport.App(bundleID: "a.b", name: "A", sourceURL: URL(string: "https://user:pw@example.com/s.json?token=x#f"))
        XCTAssertEqual(app.sourceURL?.absoluteString, "https://example.com/s.json")
    }

    /// The server rejects unknown fields, so these names must match docs/API.md.
    func testReportUsesExactlyTheDocumentedFields() throws {
        let report = DeviceReport(
            reportedAt: Date(timeIntervalSince1970: 1_795_000_000), vpnOk: true, pairingOk: false,
            appleAccount: .init(teamID: "ABCDE12345", appleID: "jane@icloud.com", appIDsUsed7d: 4),
            apps: [.init(bundleID: "a.b", name: "App", version: "1", expiresAt: Date(timeIntervalSince1970: 0),
                         sourceURL: URL(string: "https://example.com/s.json"))],
            event: .init(kind: .refresh, result: .failure, trigger: .shortcut, errorCode: .vpnOff, durationMs: 5))
        let data = try report.encoded()
        let json = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])

        XCTAssertEqual(Set(json.keys), ["reported_at", "vpn_ok", "pairing_ok", "apple_account", "apps", "event"])
        let account = try XCTUnwrap(json["apple_account"] as? [String: Any])
        XCTAssertEqual(Set(account.keys), ["team_id", "label", "app_ids_used_7d"])
        XCTAssertEqual(account["label"] as? String, "j***@icloud.com")
        let app = try XCTUnwrap((json["apps"] as? [[String: Any]])?.first)
        XCTAssertEqual(Set(app.keys), ["bundle_id", "name", "version", "expires_at", "source_url"])
        XCTAssertEqual(app["expires_at"] as? String, "1970-01-01T00:00:00Z")
        let event = try XCTUnwrap(json["event"] as? [String: Any])
        XCTAssertEqual(Set(event.keys), ["kind", "result", "trigger", "error_code", "duration_ms"])
        XCTAssertEqual(event["error_code"] as? String, "VPN_OFF")
        XCTAssertFalse(String(decoding: data, as: UTF8.self).contains("jane"))
    }
}
