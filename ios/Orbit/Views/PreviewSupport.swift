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

extension BrowseModel {
    // Not "preview": that is already a property of this model.
    static var sample: BrowseModel {
        BrowseModel(client: { nil }, sessionEnded: {})
    }
}

extension DeviceModel {
    static var preview: DeviceModel {
        DeviceModel(client: { nil }, registration: {
            DeviceRegistration(name: "iPhone", model: "iPhone16,1", iosVersion: "18.0", clientID: "preview-client")
        })
    }
}
