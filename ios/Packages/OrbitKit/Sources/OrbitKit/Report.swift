import Foundation

/// The status report. The server rejects any field it doesn't know, and these
/// types are the complete list of what leaves the phone. There is deliberately
/// nowhere to put an Apple credential.
public struct DeviceReport: Encodable, Equatable, Sendable {
    public struct AppleAccount: Encodable, Equatable, Sendable {
        public var teamID: String?
        /// Already masked: see `AppleAccount.mask`.
        public let label: String?
        public var appIDsUsed7d: Int?

        /// `appleID` is masked here, so an unmasked one can't be stored in a report.
        public init(teamID: String?, appleID: String?, appIDsUsed7d: Int?) {
            self.teamID = teamID
            self.label = appleID.map(Self.mask)
            self.appIDsUsed7d = appIDsUsed7d
        }

        /// "jane@icloud.com" becomes "j***@icloud.com".
        public static func mask(_ appleID: String) -> String {
            let parts = appleID.split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false)
            let first = parts.first.flatMap(\.first).map(String.init) ?? ""
            let local = first == "*" ? "***" : first + "***"
            return parts.count == 2 ? local + "@" + parts[1] : local
        }

        enum CodingKeys: String, CodingKey {
            case teamID = "team_id"
            case label
            case appIDsUsed7d = "app_ids_used_7d"
        }
    }

    public struct App: Encodable, Equatable, Sendable {
        public var bundleID: String
        public var name: String
        public var version: String?
        public var expiresAt: Date?
        public let sourceURL: URL?

        public init(bundleID: String, name: String, version: String? = nil, expiresAt: Date? = nil, sourceURL: URL? = nil) {
            self.bundleID = bundleID
            self.name = name
            self.version = version
            self.expiresAt = expiresAt
            self.sourceURL = sourceURL.flatMap(Self.withoutSecrets)
        }

        /// A source address may embed a login or a private token. Neither may
        /// leave the phone.
        static func withoutSecrets(_ url: URL) -> URL? {
            var parts = URLComponents(url: url, resolvingAgainstBaseURL: false)
            parts?.user = nil
            parts?.password = nil
            parts?.query = nil
            parts?.fragment = nil
            return parts?.url
        }

        enum CodingKeys: String, CodingKey {
            case bundleID = "bundle_id"
            case name, version
            case expiresAt = "expires_at"
            case sourceURL = "source_url"
        }
    }

    public struct Event: Encodable, Equatable, Sendable {
        public enum Kind: String, Encodable, Sendable { case refresh, install, remove, repair }
        public enum Result: String, Encodable, Sendable { case success, failure }
        public enum Trigger: String, Encodable, Sendable { case background, shortcut, manual, launch, setup }
        /// A code, never free text: error messages can quote account details.
        public enum ErrorCode: String, Encodable, Sendable {
            case vpnOff = "VPN_OFF"
            case pairingInvalid = "PAIRING_INVALID"
            case appleReauth2FA = "APPLE_REAUTH_2FA"
            case appIDLimit = "APP_ID_LIMIT"
            case anisetteUnavailable = "ANISETTE_UNAVAILABLE"
            case unknown = "UNKNOWN"
        }

        public var kind: Kind
        public var result: Result
        public var trigger: Trigger?
        public var errorCode: ErrorCode?
        public var durationMs: Int?

        public init(kind: Kind, result: Result, trigger: Trigger? = nil, errorCode: ErrorCode? = nil, durationMs: Int? = nil) {
            self.kind = kind
            self.result = result
            self.trigger = trigger
            self.errorCode = errorCode
            self.durationMs = durationMs
        }

        enum CodingKeys: String, CodingKey {
            case kind, result, trigger
            case errorCode = "error_code"
            case durationMs = "duration_ms"
        }
    }

    public var reportedAt: Date
    public var vpnOk: Bool?
    public var pairingOk: Bool?
    public var appleAccount: AppleAccount?
    /// nil leaves the server's list unchanged; empty means no apps.
    public var apps: [App]?
    public var event: Event?

    public init(reportedAt: Date, vpnOk: Bool? = nil, pairingOk: Bool? = nil, appleAccount: AppleAccount? = nil, apps: [App]? = nil, event: Event? = nil) {
        self.reportedAt = reportedAt
        self.vpnOk = vpnOk
        self.pairingOk = pairingOk
        self.appleAccount = appleAccount
        self.apps = apps
        self.event = event
    }

    enum CodingKeys: String, CodingKey {
        case reportedAt = "reported_at"
        case vpnOk = "vpn_ok"
        case pairingOk = "pairing_ok"
        case appleAccount = "apple_account"
        case apps, event
    }

    public func encoded() throws -> Data {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        return try encoder.encode(self)
    }
}
