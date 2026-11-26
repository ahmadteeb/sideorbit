import OrbitKit
import SwiftUI

struct SignInView: View {
    @ObservedObject var model: SignInModel
    @State private var web = WebAuth()
    @FocusState private var focus: Field?

    private enum Field { case server, email }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: OrbitSpace.xl) {
                header
                server
                if let methods = model.methods {
                    choices(methods)
                }
                if let message = model.errorMessage {
                    ErrorLabel(message: message)
                }
                if model.busy {
                    ProgressView("Working…")
                        .frame(maxWidth: .infinity)
                }
            }
            .frame(maxWidth: 480)
            .frame(maxWidth: .infinity)
            .padding(.horizontal, OrbitSpace.l)
            .padding(.vertical, OrbitSpace.xxl)
        }
        .background(OrbitColor.bg.ignoresSafeArea())
        // A saved server loads its sign-in options by itself.
        .task {
            if model.methods == nil && !model.serverText.isEmpty {
                await model.connect()
            }
        }
        .onChange(of: model.errorMessage) { message in
            if let message {
                UIAccessibility.post(notification: .announcement, argument: message)
            }
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: OrbitSpace.s) {
            Text("Sign in to Orbit")
                .font(.largeTitle.weight(.semibold))
                .foregroundStyle(OrbitColor.text)
                .accessibilityAddTraits(.isHeader)
            Text("This is your Orbit account, not your Apple ID.")
                .foregroundStyle(OrbitColor.textMuted)
        }
    }

    private var server: some View {
        VStack(alignment: .leading, spacing: OrbitSpace.s) {
            Text("Server address")
                .font(.headline)
                .foregroundStyle(OrbitColor.text)
            field("orbit.example.com", text: $model.serverText, label: "Server address")
                .keyboardType(.URL)
                .textContentType(.URL)
                .focused($focus, equals: .server)
                .submitLabel(.continue)
                .onSubmit { Task { await model.connect() } }
            Button("Continue") {
                focus = nil
                Task { await model.connect() }
            }
            .buttonStyle(OrbitSecondaryStyle())
            .disabled(model.busy || model.serverText.isEmpty)
        }
    }

    @ViewBuilder
    private func choices(_ methods: SignInMethods) -> some View {
        VStack(alignment: .leading, spacing: OrbitSpace.m) {
            if !methods.github && !methods.google && !methods.email {
                Text("This server has no way to sign in set up. Ask whoever runs it.")
                    .foregroundStyle(OrbitColor.textMuted)
            }
            // Text-only until the official brand assets are added (D29).
            if methods.github {
                Button("Continue with GitHub") { Task { await model.signIn(with: .github, using: web) } }
                    .buttonStyle(OrbitSecondaryStyle())
            }
            if methods.google {
                Button("Continue with Google") { Task { await model.signIn(with: .google, using: web) } }
                    .buttonStyle(OrbitSecondaryStyle())
            }
            if methods.email {
                if model.emailSent {
                    Label("Check your inbox. Open the link on this iPhone within 15 minutes.", systemImage: "envelope.badge")
                        .foregroundStyle(OrbitColor.text)
                    Button("Use a different address") { model.useDifferentEmail() }
                        .buttonStyle(OrbitSecondaryStyle())
                } else {
                    Text("Email")
                        .font(.headline)
                        .foregroundStyle(OrbitColor.text)
                    field("name@example.com", text: $model.email, label: "Email")
                        .keyboardType(.emailAddress)
                        .textContentType(.emailAddress)
                        .focused($focus, equals: .email)
                        .submitLabel(.send)
                        .onSubmit { Task { await model.sendEmailLink() } }
                    Button("Email me a sign-in link") {
                        focus = nil
                        Task { await model.sendEmailLink() }
                    }
                    .buttonStyle(OrbitPrimaryStyle())
                }
            }
        }
        .disabled(model.busy)
    }

    private func field(_ placeholder: String, text: Binding<String>, label: String) -> some View {
        TextField(placeholder, text: text)
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .accessibilityLabel(label)
            .padding(OrbitSpace.m)
            .frame(minHeight: 44)
            .foregroundStyle(OrbitColor.text)
            .background(OrbitColor.surface2, in: RoundedRectangle(cornerRadius: OrbitRadius.sm))
            .overlay(RoundedRectangle(cornerRadius: OrbitRadius.sm).strokeBorder(OrbitColor.borderStrong))
    }
}

#Preview("Light") {
    SignInView(model: .preview)
        .preferredColorScheme(.light)
}

#Preview("Dark") {
    SignInView(model: .preview)
        .preferredColorScheme(.dark)
}

#Preview("Largest text") {
    SignInView(model: .preview)
        .environment(\.dynamicTypeSize, .accessibility5)
}
