import OrbitKit
import SwiftUI

@main
struct OrbitApp: App {
    @StateObject private var model = OrbitApp.makeModel()

    var body: some Scene {
        WindowGroup {
            RootView(model: model)
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
