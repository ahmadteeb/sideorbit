import OrbitKit
import SwiftUI

/// The four tabs of design-system/orbit/pages/ios.md. Each keeps its own stack.
struct MainTabView: View {
    @ObservedObject var model: SignInModel
    @ObservedObject var device: DeviceModel
    @StateObject private var browse: BrowseModel
    @Environment(\.scenePhase) private var scenePhase

    init(model: SignInModel, device: DeviceModel) {
        self.model = model
        self.device = device
        _browse = StateObject(wrappedValue: BrowseModel(
            client: { model.currentClient() }, sessionEnded: { await model.sessionEnded() }))
    }

    var body: some View {
        TabView {
            NavigationStack {
                HomeView(device: device, server: ServerAddress.parse(model.serverText))
            }
            .tabItem { Label("Home", systemImage: "house").environment(\.symbolVariants, .none) }

            NavigationStack {
                BrowseView(model: browse)
            }
            .tabItem { Label("Browse", systemImage: "square.grid.2x2").environment(\.symbolVariants, .none) }

            NavigationStack {
                placeholder("clock.arrow.circlepath", "Nothing yet", "Refresh history will appear here.")
                    .navigationTitle("Activity")
            }
            .tabItem { Label("Activity", systemImage: "clock.arrow.circlepath").environment(\.symbolVariants, .none) }

            NavigationStack {
                SettingsView(model: model)
            }
            .tabItem { Label("Settings", systemImage: "gearshape").environment(\.symbolVariants, .none) }
        }
        .tint(OrbitColor.primary)
        .task { await device.checkIn() }
        .onChange(of: scenePhase) { phase in
            if phase == .active {
                Task { await device.checkIn() }
            }
        }
    }

    private func placeholder(_ symbol: String, _ title: String, _ message: String) -> some View {
        EmptyStateView(symbol: symbol, title: title, message: message) {
            if let url = ServerAddress.parse(model.serverText) {
                Link("Open the web dashboard", destination: url)
                    .buttonStyle(OrbitSecondaryStyle())
            }
        }
    }
}

#Preview("Dark") {
    MainTabView(model: .preview, device: .preview)
        .preferredColorScheme(.dark)
}

#Preview("Light, largest text") {
    MainTabView(model: .preview, device: .preview)
        .environment(\.dynamicTypeSize, .accessibility5)
}
