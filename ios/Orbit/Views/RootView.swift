import OrbitKit
import SwiftUI

struct RootView: View {
    @ObservedObject var model: SignInModel

    var body: some View {
        if model.signedIn {
            MainTabView(model: model)
        } else {
            SignInView(model: model)
        }
    }
}
