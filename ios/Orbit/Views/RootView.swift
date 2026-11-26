import OrbitKit
import SwiftUI

struct RootView: View {
    @ObservedObject var model: SignInModel
    @ObservedObject var device: DeviceModel

    var body: some View {
        Group {
            if model.signedIn {
                MainTabView(model: model, device: device)
            } else {
                SignInView(model: model)
            }
        }
        .onChange(of: model.signedIn) { signedIn in
            if !signedIn { device.forget() }
        }
    }
}
