import OrbitKit
import SwiftUI

struct SettingsView: View {
    @ObservedObject var model: SignInModel
    @State private var profile: Profile?
    @State private var errorMessage: String?
    @State private var confirmingSignOut = false

    var body: some View {
        List {
            Section("Account") {
                if let profile {
                    LabeledContent("Email", value: profile.email ?? "Unknown")
                } else if let errorMessage {
                    ErrorLabel(message: errorMessage)
                    Button("Retry") { Task { await load() } }
                } else {
                    ProgressView("Loading…")
                }
                LabeledContent("Server") {
                    Text(model.serverText).font(.footnote.monospaced())
                }
                Button("Sign out", role: .destructive) { confirmingSignOut = true }
            }

            Section("About") {
                Text("Orbit is free software under the AGPL-3.0.")
                link("Source code", "https://github.com/ahmadteeb/sideorbit")
                link("SideStore", "https://github.com/SideStore/SideStore")
                link("iloader", "https://github.com/nab138/iloader")
                link("anisette-v3-server", "https://github.com/Dadoum/anisette-v3-server")
                link("LiveContainer", "https://github.com/LiveContainer/LiveContainer")
            }
        }
        .navigationTitle("Settings")
        .task { await load() }
        .confirmationDialog(
            "Sign out of \(model.serverText)?", isPresented: $confirmingSignOut, titleVisibility: .visible
        ) {
            Button("Sign out", role: .destructive) { Task { await model.signOut() } }
        } message: {
            Text("Your Orbit account on this iPhone is signed out. Installed apps stay.")
        }
    }

    @ViewBuilder
    private func link(_ title: String, _ address: String) -> some View {
        if let url = URL(string: address) {
            Link(title, destination: url)
        }
    }

    private func load() async {
        errorMessage = nil
        do {
            profile = try await model.currentClient()?.profile()
        } catch APIError.signedOut {
            await model.signOut()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

#Preview {
    NavigationStack {
        SettingsView(model: .preview)
    }
}
