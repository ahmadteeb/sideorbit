import Foundation
import Security

/// Tokens and the server that issued them. They must never be sent anywhere else.
public struct StoredSession: Codable, Equatable, Sendable {
    public var server: URL
    public var tokens: Tokens

    public init(server: URL, tokens: Tokens) {
        self.server = server
        self.tokens = tokens
    }
}

public protocol TokenStore: Sendable {
    func load() -> StoredSession?
    func save(_ session: StoredSession) throws
    func clear()
}

public struct KeychainError: Error, Equatable {
    public let status: OSStatus
}

/// Orbit's own session tokens, in the app's Keychain. Readable after the first
/// unlock so a background refresh can report, and never synced or backed up to
/// another device.
public struct KeychainTokenStore: TokenStore {
    private let service: String

    public init(service: String = "dev.orbitstore.Orbit.session") {
        self.service = service
    }

    private var query: [CFString: Any] {
        [kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: "session"]
    }

    public func load() -> StoredSession? {
        var search = query
        search[kSecReturnData] = true
        search[kSecMatchLimit] = kSecMatchLimitOne
        var item: CFTypeRef?
        guard SecItemCopyMatching(search as CFDictionary, &item) == errSecSuccess, let data = item as? Data else {
            return nil
        }
        return try? JSONDecoder().decode(StoredSession.self, from: data)
    }

    public func save(_ session: StoredSession) throws {
        let data = try JSONEncoder().encode(session)
        let values: [CFString: Any] = [
            kSecValueData: data,
            kSecAttrAccessible: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
        ]
        var status = SecItemUpdate(query as CFDictionary, values as CFDictionary)
        if status == errSecItemNotFound {
            status = SecItemAdd(query.merging(values) { _, new in new } as CFDictionary, nil)
        }
        guard status == errSecSuccess else { throw KeychainError(status: status) }
    }

    public func clear() {
        SecItemDelete(query as CFDictionary)
    }
}
