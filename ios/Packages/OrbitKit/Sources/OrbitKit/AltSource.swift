import Foundation

/// An AltStore-format app source. It's third-party content: parsing is tolerant
/// (a bad app is skipped, not the whole source) and every string is bounded.
public struct AltSource: Equatable, Sendable {
    public struct App: Identifiable, Equatable, Hashable, Sendable {
        public var id: String { bundleID }
        public let bundleID: String
        public let name: String
        public let developer: String?
        public let summary: String?
        public let version: String?
        public let size: Int?
        public let minOSVersion: String?
        public let downloadURL: URL
        public let entitlements: [String]
        /// Usage descriptions the app declares, such as camera access.
        public let privacyKeys: [String]

        /// Features this app asks for that a free Apple ID can't grant.
        public var unavailableOnFreeAccounts: [String] {
            var names: [String] = []
            for entitlement in entitlements {
                let name: String?
                switch entitlement {
                case "aps-environment": name = "Push notifications"
                case "com.apple.developer.associated-domains": name = "Associated domains"
                case "com.apple.developer.applesignin": name = "Sign in with Apple"
                case "com.apple.developer.networking.networkextension": name = "Network extensions"
                case let e where e.hasPrefix("com.apple.developer.icloud") || e.hasPrefix("com.apple.developer.ubiquity"):
                    name = "iCloud"
                default: name = nil
                }
                if let name, !names.contains(name) { names.append(name) }
            }
            return names
        }
    }

    public let name: String
    public let apps: [App]

    public struct Invalid: Error, LocalizedError {
        public var errorDescription: String? { "That address doesn't hold an app source Orbit can read." }
    }

    public static func parse(_ data: Data) throws -> AltSource {
        guard let root = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let name = text(root["name"], limit: 200),
              let entries = root["apps"] as? [Any]
        else { throw Invalid() }
        var seen = Set<String>()
        var apps: [App] = []
        for case let entry as [String: Any] in entries.prefix(500) {
            if let app = app(entry), seen.insert(app.bundleID).inserted {
                apps.append(app)
            }
        }
        return AltSource(name: name, apps: apps)
    }

    private static func app(_ entry: [String: Any]) -> App? {
        // The newest version comes first. Older sources put these on the app.
        let latest = (entry["versions"] as? [[String: Any]])?.first ?? entry
        guard let bundleID = text(entry["bundleIdentifier"], limit: 155),
              let name = text(entry["name"], limit: 200),
              let address = text(latest["downloadURL"], limit: 2048),
              let download = URL(string: address), download.scheme?.lowercased() == "https", download.host != nil
        else { return nil }
        let permissions = entry["appPermissions"] as? [String: Any]
        return App(
            bundleID: bundleID, name: name,
            developer: text(entry["developerName"], limit: 200),
            summary: text(entry["localizedDescription"], limit: 4000) ?? text(entry["subtitle"], limit: 200),
            version: text(latest["version"], limit: 64),
            size: (latest["size"] as? NSNumber).map(\.intValue).flatMap { $0 > 0 ? $0 : nil },
            minOSVersion: text(latest["minOSVersion"], limit: 32),
            downloadURL: download,
            entitlements: names(permissions?["entitlements"]),
            privacyKeys: names(permissions?["privacy"]))
    }

    /// Accepts ["a"], [{"name": "a"}] or {"a": "reason"}.
    private static func names(_ value: Any?) -> [String] {
        let found: [String]
        if let list = value as? [Any] {
            found = list.compactMap { text($0, limit: 200) ?? text(($0 as? [String: Any])?["name"], limit: 200) }
        } else if let map = value as? [String: Any] {
            found = map.keys.sorted()
        } else {
            found = []
        }
        return Array(found.prefix(100))
    }

    private static func text(_ value: Any?, limit: Int) -> String? {
        guard let string = (value as? String)?.trimmingCharacters(in: .whitespacesAndNewlines), !string.isEmpty else {
            return nil
        }
        return String(string.prefix(limit))
    }
}
