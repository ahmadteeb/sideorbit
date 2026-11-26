import OrbitKit
import SwiftUI

@main
struct OrbitApp: App {
    @StateObject private var model: SignInModel
    @StateObject private var device: DeviceModel

    init() {
        let model = OrbitApp.makeModel()
        _model = StateObject(wrappedValue: model)
        _device = StateObject(wrappedValue: DeviceModel(
            client: { model.currentClient() }, registration: ThisDevice.registration))
    }

    var body: some Scene {
        WindowGroup {
            RootView(model: model, device: device)
                // Emailed sign-in links come back through the orbit:// scheme.
                .onOpenURL { url in
                    Task { await model.handle(callback: url) }
                }
        }
    }

    @MainActor
    private static func makeModel() -> SignInModel {
        let store = KeychainTokenStore()
        return SignInModel(sessionServer: store.load()?.server) { url in
            APIClient(server: url, store: store)
        }
    }
}
