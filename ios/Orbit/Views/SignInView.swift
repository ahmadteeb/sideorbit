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
                if let notice = model.notice {
                    Label(notice, systemImage: "info.circle")
                        .foregroundStyle(OrbitColor.info)
                }
                server
                if let methods = model.methods {
                    choices(methods)
                }
            }
            .frame(maxWidth: 480)
            .frame(maxWidth: .infinity)
            .padding(.horizontal, OrbitSpace.l)
            .padding(.vertical, OrbitSpace.xxl)
        }
        .background(OrbitColor.bg.ignoresSafeArea())
        // Keeps scrolled content from running under the status bar.
        .safeAreaInset(edge: .top, spacing: 0) {
            Color.clear.frame(height: 0).background(OrbitColor.bg)
        }
        // A saved server loads its sign-in options by itself.
        .task {
            if model.methods == nil && !model.serverText.isEmpty {
                await model.connect()
            }
        }
        .onChange(of: model.errorCount) { _ in
            if let message = model.errorMessage {
                announce(message)
            }
        }
        .onChange(of: model.emailSent) { sent in
            if sent {
                announce("Check your inbox.")
            }
        }
        // Validate the field being left.
        .onChange(of: focus) { [previous = focus] next in
            if previous == .server && next != .server { model.validateServer() }
            if previous == .email && next != .email && !model.email.isEmpty { model.validateEmail() }
        }
    }

    /// Progress and errors sit with the controls that caused them.
    @ViewBuilder
    private func status(_ area: SignInModel.Area) -> some View {
        if let message = model.errorMessage, model.errorArea == area {
            ErrorLabel(message: message)
        }
    }

    @ViewBuilder
    private var progress: some View {
        if let activity = model.activity {
            ProgressView(activity)
                .frame(maxWidth: .infinity)
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
            field("orbit.example.com", text: $model.serverText, label: "Server address", id: .server)
                .keyboardType(.URL)
                .textContentType(.URL)
                .submitLabel(.continue)
                .onSubmit { connect() }
                .disabled(model.busy)
            status(.server)
            Button("Continue") { connect() }
                .buttonStyle(OrbitSecondaryStyle())
                .disabled(model.busy || model.serverText.isEmpty)
            if model.methods == nil {
                progress
            }
        }
    }

    private func connect() {
        guard !model.busy else { return }
        focus = nil
        Task { await model.connect() }
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
            status(.providers)
            progress
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
                    field("name@example.com", text: $model.email, label: "Email", id: .email)
                        .keyboardType(.emailAddress)
                        .textContentType(.emailAddress)
                        .submitLabel(.send)
                        .onSubmit { sendLink() }
                    status(.email)
                    Button("Email me a sign-in link") { sendLink() }
                        .buttonStyle(OrbitPrimaryStyle())
                }
            }
        }
        .disabled(model.busy)
    }

    private func sendLink() {
        focus = nil
        Task { await model.sendEmailLink() }
    }

    private func field(_ placeholder: String, text: Binding<String>, label: String, id: Field) -> some View {
        TextField(placeholder, text: text)
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .accessibilityLabel(label)
            .focused($focus, equals: id)
            .padding(OrbitSpace.m)
            .frame(minHeight: 44)
            .foregroundStyle(OrbitColor.text)
            .background(OrbitColor.surface2, in: RoundedRectangle(cornerRadius: OrbitRadius.sm))
            .overlay(RoundedRectangle(cornerRadius: OrbitRadius.sm).strokeBorder(OrbitColor.borderStrong))
            // A focus ring outside the field.
            .overlay(
                RoundedRectangle(cornerRadius: OrbitRadius.sm + 2)
                    .stroke(OrbitColor.info, lineWidth: focus == id ? 2 : 0)
                    .padding(-3))
            // The whole box focuses the field, not just its line of text.
            .contentShape(Rectangle())
            .onTapGesture { focus = id }
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
