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
