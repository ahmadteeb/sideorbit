import Foundation
import Security

public protocol TokenStore: Sendable {
    func load() -> Tokens?
    func save(_ tokens: Tokens) throws
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
        [kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: "tokens"]
    }

    public func load() -> Tokens? {
        var search = query
        search[kSecReturnData] = true
        search[kSecMatchLimit] = kSecMatchLimitOne
        var item: CFTypeRef?
        guard SecItemCopyMatching(search as CFDictionary, &item) == errSecSuccess, let data = item as? Data else {
            return nil
        }
        return try? JSONDecoder().decode(Tokens.self, from: data)
    }

    public func save(_ tokens: Tokens) throws {
        let data = try JSONEncoder().encode(tokens)
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
