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

/// One value in the app's Keychain. Readable after the first unlock so a
/// background task can use it, and never synced or restored to another device.
public struct KeychainItem: Sendable {
    private let service: String
    private let account: String

    public init(service: String, account: String) {
        self.service = service
        self.account = account
    }

    private var query: [CFString: Any] {
        [kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: account]
    }

    public func load() -> Data? {
        var search = query
        search[kSecReturnData] = true
        search[kSecMatchLimit] = kSecMatchLimitOne
        var item: CFTypeRef?
        guard SecItemCopyMatching(search as CFDictionary, &item) == errSecSuccess else { return nil }
        return item as? Data
    }

    public func save(_ data: Data) throws {
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

/// Orbit's own session tokens.
public struct KeychainTokenStore: TokenStore {
    private let item: KeychainItem

    public init(service: String = "dev.orbitstore.Orbit.session") {
        item = KeychainItem(service: service, account: "session")
    }

    public func load() -> StoredSession? {
        item.load().flatMap { try? JSONDecoder().decode(StoredSession.self, from: $0) }
    }

    public func save(_ session: StoredSession) throws {
        try item.save(JSONEncoder().encode(session))
    }

    public func clear() {
        item.clear()
    }
}

/// This installation's random identifier, created once. It is the same kind of
/// value an anisette v3 client uses, so Apple sign-in can reuse it later.
public enum InstallationID {
    public static func loadOrCreate(
        _ item: KeychainItem = KeychainItem(service: "dev.orbitstore.Orbit.install", account: "client-id")
    ) throws -> String {
        if let data = item.load(), let id = String(data: data, encoding: .utf8), !id.isEmpty {
            return id
        }
        let id = Data((0..<16).map { _ in UInt8.random(in: .min ... .max) }).base64EncodedString()
        try item.save(Data(id.utf8))
        return id
    }
}
