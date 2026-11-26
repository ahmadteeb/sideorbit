import OrbitKit

extension SignInModel {
    /// A model for previews, with nothing stored in the Keychain.
    static var preview: SignInModel {
        SignInModel { url in APIClient(server: url, store: PreviewStore()) }
    }
}

private struct PreviewStore: TokenStore {
    func load() -> Tokens? { nil }
    func save(_ tokens: Tokens) throws {}
    func clear() {}
}
