import Foundation

// Shapes from docs/API.md.

public struct Tokens: Codable, Equatable, Sendable {
    public var accessToken: String
    public var refreshToken: String

    public init(accessToken: String, refreshToken: String) {
        self.accessToken = accessToken
        self.refreshToken = refreshToken
    }

    enum CodingKeys: String, CodingKey {
        case accessToken = "access_token"
        case refreshToken = "refresh_token"
    }
}

public struct SignInMethods: Codable, Equatable, Sendable {
    public var github: Bool
    public var google: Bool
    public var email: Bool
}

public struct Profile: Codable, Equatable, Sendable {
    public var id: String
    public var email: String?
    public var identities: [String]
}

public struct SavedSource: Codable, Equatable, Hashable, Identifiable, Sendable {
    public var id: String
    public var url: String
    public var name: String?

    public init(id: String, url: String, name: String?) {
        self.id = id
        self.url = url
        self.name = name
    }

    public var title: String { name ?? url }
}

public struct SourcePreview: Codable, Equatable, Sendable {
    public var url: String
    public var name: String
    public var appCount: Int
    public var apps: [String]

    enum CodingKeys: String, CodingKey {
        case url, name, apps
        case appCount = "app_count"
    }
}

public struct DeviceRegistration: Encodable, Equatable, Sendable {
    public var name: String
    public var model: String
    public var iosVersion: String
    public var clientID: String

    public init(name: String, model: String, iosVersion: String, clientID: String) {
        self.name = name
        self.model = model
        self.iosVersion = iosVersion
        self.clientID = clientID
    }

    enum CodingKeys: String, CodingKey {
        case name, model
        case iosVersion = "ios_version"
        case clientID = "client_id"
    }
}

public struct RegisteredDevice: Codable, Equatable, Sendable {
    public var id: String
    public var name: String
}
