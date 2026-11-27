import XCTest
@testable import OrbitKit

final class SourceTests: XCTestCase {
    private let modern = #"""
    {"name":"Test","identifier":"x","apps":[
      {"name":"Player","bundleIdentifier":"com.example.player","developerName":"Ada","localizedDescription":"Plays.",
       "versions":[{"version":"2.0","size":1048576,"minOSVersion":"16.0","downloadURL":"https://dl.example.com/p2.ipa"},
                   {"version":"1.0","downloadURL":"https://dl.example.com/p1.ipa"}],
       "appPermissions":{"entitlements":["aps-environment",{"name":"com.apple.developer.icloud-services"},"get-task-allow"],
                          "privacy":{"NSCameraUsageDescription":"scan"}}},
      {"name":"Old style","bundleIdentifier":"com.example.old","version":"0.9","downloadURL":"https://dl.example.com/o.ipa",
       "appPermissions":{"privacy":[{"name":"Microphone","usageDescription":"x"}]}},
      {"name":"No download","bundleIdentifier":"com.example.none"},
      {"name":"Plain http","bundleIdentifier":"com.example.http","downloadURL":"http://dl.example.com/h.ipa"},
      {"name":"Duplicate","bundleIdentifier":"com.example.player","downloadURL":"https://dl.example.com/d.ipa"},
      "junk"
    ]}
    """#

    func testParsesNewAndOldShapesAndSkipsBadApps() throws {
        let source = try AltSource.parse(Data(modern.utf8))
        XCTAssertEqual(source.name, "Test")
        XCTAssertEqual(source.apps.map(\.bundleID), ["com.example.player", "com.example.old"])
        let player = source.apps[0]
        XCTAssertEqual(player.version, "2.0")
        XCTAssertEqual(player.size, 1_048_576)
        XCTAssertEqual(player.downloadURL.absoluteString, "https://dl.example.com/p2.ipa")
        XCTAssertEqual(player.unavailableOnFreeAccounts, ["Push notifications", "iCloud"])
        XCTAssertEqual(player.otherEntitlements, ["get-task-allow"])
        XCTAssertEqual(player.privacyKeys, ["NSCameraUsageDescription"])
        XCTAssertEqual(source.apps[1].privacyKeys, ["Microphone"])
    }

    func testRejectsWhatIsNotASource() {
        for text in ["", "<html>", "[]", #"{"apps":[]}"#, #"{"name":"x"}"#] {
            XCTAssertThrowsError(try AltSource.parse(Data(text.utf8)), text)
        }
    }

    func testFetcherSendsNoTokenAndBoundsTheSize() async throws {
        let transport = StubTransport { request in
            if request.url!.path == "/big" {
                return (200, #"{"name":"x","apps":[],"pad":""# + String(repeating: "a", count: SourceFetcher.maxBytes) + #""}"#)
            }
            return request.url!.path == "/s.json" ? (200, #"{"name":"S","apps":[]}"#) : (404, "")
        }
        let fetcher = SourceFetcher(transport: transport)
        let source = try await fetcher.fetch(URL(string: "https://example.com/s.json")!)
        XCTAssertEqual(source.name, "S")
        XCTAssertNil(transport.requests[0].value(forHTTPHeaderField: "Authorization"))
        for address in ["https://example.com/big", "https://example.com/missing", "http://example.com/s.json"] {
            do {
                _ = try await fetcher.fetch(URL(string: address)!)
                XCTFail(address)
            } catch {}
        }
    }

    func testExpiryComesFromTheProvisioningProfile() throws {
        let expires = Date(timeIntervalSince1970: 1_796_000_000)
        let plist = try PropertyListSerialization.data(fromPropertyList: ["ExpirationDate": expires, "Name": "x"], format: .xml, options: 0)
        // The real file wraps the list in a signature.
        let profile = Data([0x30, 0x82, 0x01]) + plist + Data([0x00, 0xff])
        XCTAssertEqual(Expiry.date(inProvisioningProfile: profile), expires)
        XCTAssertNil(Expiry.date(inProvisioningProfile: Data("nothing".utf8)))
    }

    func testStatusAndReminders() {
        let now = Date(timeIntervalSince1970: 1_000_000)
        let hour = 3600.0
        XCTAssertEqual(Expiry.status(now.addingTimeInterval(120 * hour), now: now), .fresh(daysLeft: 5))
        XCTAssertEqual(Expiry.status(now.addingTimeInterval(31 * hour), now: now), .expiring(hoursLeft: 31))
        XCTAssertEqual(Expiry.status(now, now: now), .expired)

        let all = Expiry.reminders(for: now.addingTimeInterval(100 * hour), now: now)
        XCTAssertEqual(all.map(\.id), Expiry.reminderIDs)
        XCTAssertEqual(all[0].date, now.addingTimeInterval(52 * hour))
        XCTAssertEqual(Expiry.reminders(for: now.addingTimeInterval(30 * hour), now: now).map(\.id), ["orbit-expiry-24", "orbit-expiry-6"])
        XCTAssertTrue(Expiry.reminders(for: now.addingTimeInterval(5 * hour), now: now).isEmpty)
    }
}
