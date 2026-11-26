import OrbitKit

extension SignInModel {
    /// A model for previews, with nothing stored in the Keychain.
    static var preview: SignInModel {
        SignInModel { url in APIClient(server: url, store: PreviewStore()) }
    }
}

private struct PreviewStore: TokenStore {
    func load() -> StoredSession? { nil }
    func save(_ session: StoredSession) throws {}
    func clear() {}
}
