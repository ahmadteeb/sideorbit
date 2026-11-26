import OrbitKit
import SwiftUI

/// The four tabs of design-system/orbit/pages/ios.md. Each keeps its own stack.
struct MainTabView: View {
    @ObservedObject var model: SignInModel

    var body: some View {
        TabView {
            NavigationStack {
                placeholder("house", "No apps yet", "Installing and refreshing apps isn't built yet.")
                    .navigationTitle("Home")
            }
            .tabItem { Label("Home", systemImage: "house").environment(\.symbolVariants, .none) }

            NavigationStack {
                placeholder("square.grid.2x2", "No sources yet", "Browsing app sources isn't built yet.")
                    .navigationTitle("Browse")
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
    MainTabView(model: .preview)
        .preferredColorScheme(.dark)
}

#Preview("Light, largest text") {
    MainTabView(model: .preview)
        .environment(\.dynamicTypeSize, .accessibility5)
}
