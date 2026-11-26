import OrbitKit
import SwiftUI

struct RootView: View {
    @ObservedObject var model: SignInModel
    @ObservedObject var device: DeviceModel

    var body: some View {
        if model.signedIn {
            MainTabView(model: model, device: device)
        } else {
            // Also covers a sign-out that happened while the app was closed.
            SignInView(model: model)
                .onAppear { device.forget() }
        }
    }
}
