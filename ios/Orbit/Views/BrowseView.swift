import OrbitKit
import SwiftUI

struct BrowseView: View {
    @ObservedObject var model: BrowseModel
    @State private var adding = false
    @State private var removing: SavedSource?

    var body: some View {
        // The modifiers must sit on a view whose identity doesn't change with the
        // content, or loading would restart whenever the branch switches.
        ZStack { content }
            .onChange(of: model.errorMessage) { message in
                if let message { announce(message) }
            }
            .navigationTitle("Browse")
            .toolbar {
                Button {
                    adding = true
                } label: {
                    Label("Add source", systemImage: "plus")
                }
            }
            .navigationDestination(for: SavedSource.self) { source in
                SourceView(model: SourceModel(source: source), browse: model)
            }
            .sheet(isPresented: $adding) {
                AddSourceView(model: model)
            }
            .confirmationDialog(
                "Remove \(removing?.title ?? "source")?", isPresented: Binding(get: { removing != nil }, set: { if !$0 { removing = nil } }),
                titleVisibility: .visible, presenting: removing
            ) { source in
                Button("Remove \(source.title)", role: .destructive) { Task { await model.remove(source) } }
            } message: { _ in
                Text("It disappears from your saved sources. Installed apps stay.")
            }
            .task { await model.load() }
    }

    @ViewBuilder
    private var content: some View {
        if let sources = model.sources, !sources.isEmpty {
            List {
                if let message = model.errorMessage {
                    Group {
                        ErrorLabel(message: message)
                        Button("Retry") { Task { await model.load() } }
                    }
                    .listRowBackground(OrbitColor.surface)
                }
                ForEach(sources) { source in
                    NavigationLink(value: source) {
                        VStack(alignment: .leading, spacing: OrbitSpace.xs) {
                            Text(source.title).foregroundStyle(OrbitColor.text)
                            Text(source.url)
                                .font(.footnote.monospaced())
                                .foregroundStyle(OrbitColor.textMuted)
                        }
                    }
                    .listRowBackground(OrbitColor.surface)
                    // Also available as a button inside the source.
                    .swipeActions {
                        Button("Remove") { removing = source }
                            .tint(OrbitColor.danger)
                    }
                }
            }
            .orbitList()
            .refreshable { await model.load() }
        } else if model.sources != nil {
            EmptyStateView(symbol: "square.grid.2x2", title: "No sources yet", message: "Add a link to an app source to see its apps.") {
                if let message = model.errorMessage {
                    ErrorLabel(message: message)
                    Button("Retry") { Task { await model.load() } }
                        .buttonStyle(OrbitSecondaryStyle())
                }
                Button("Add source") { adding = true }
                    .buttonStyle(OrbitPrimaryStyle())
            }
        } else if let message = model.errorMessage {
            EmptyStateView(symbol: "exclamationmark.triangle", title: "Couldn't load sources", message: message) {
                Button("Retry") { Task { await model.load() } }
                    .buttonStyle(OrbitSecondaryStyle())
            }
        } else {
            ProgressView("Loading sources…")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(OrbitColor.bg.ignoresSafeArea())
        }
    }
}

struct AddSourceView: View {
    @ObservedObject var model: BrowseModel
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("https://example.com/source.json", text: $model.address)
                        .keyboardType(.URL)
                        .textContentType(.URL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .accessibilityLabel("Source address")
                        .onSubmit { Task { await model.check() } }
                    if let message = model.addError {
                        ErrorLabel(message: message)
                    }
                    Button("Check source") { Task { await model.check() } }
                        .disabled(model.busy || model.address.isEmpty)
                } header: {
                    Text("Source address").foregroundStyle(OrbitColor.textMuted)
                }
                .listRowBackground(OrbitColor.surface)

                if model.busy {
                    ProgressView("Working…")
                        .listRowBackground(OrbitColor.surface)
                }

                if let preview = model.preview {
                    Section {
                        Text(preview.name).font(.headline)
                        Text(preview.appCount == 1 ? "1 app" : "\(preview.appCount) apps")
                        Text("Only add sources you trust. Orbit doesn't check their apps.")
                            .foregroundStyle(OrbitColor.textMuted)
                        Button("Save source") {
                            Task {
                                if await model.save() { dismiss() }
                            }
                        }
                        .disabled(model.busy)
                    }
                    .listRowBackground(OrbitColor.surface)
                }
            }
            .orbitList()
            .tint(OrbitColor.info)
            .onAppear { model.resetAdd() }
            .onChange(of: model.addError) { message in
                if let message { announce(message) }
            }
            .onChange(of: model.preview) { preview in
                if let preview { announce("Found \(preview.name)") }
            }
            .navigationTitle("Add source")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
        }
    }
}

struct SourceView: View {
    @StateObject private var model: SourceModel
    @ObservedObject private var browse: BrowseModel
    @Environment(\.dismiss) private var dismiss
    @State private var confirming = false

    init(model: @autoclosure @escaping () -> SourceModel, browse: BrowseModel) {
        _model = StateObject(wrappedValue: model())
        self.browse = browse
    }

    var body: some View {
        ZStack {
            if let contents = model.contents, contents.apps.isEmpty {
                EmptyStateView(symbol: "square.grid.2x2", title: "No apps here", message: "This source lists no apps Orbit can read.") {
                    Button("Check again") { Task { await model.load() } }
                        .buttonStyle(OrbitSecondaryStyle())
                }
            } else {
                apps
            }
        }
        .navigationTitle(model.source.title)
        .toolbar {
            Button {
                confirming = true
            } label: {
                Label("Remove source", systemImage: "trash")
            }
            .tint(OrbitColor.danger)
        }
        .confirmationDialog("Remove \(model.source.title)?", isPresented: $confirming, titleVisibility: .visible) {
            Button("Remove \(model.source.title)", role: .destructive) {
                Task {
                    // Stay on the screen if it didn't work, so the error is seen.
                    if await browse.remove(model.source) { dismiss() }
                }
            }
        } message: {
            Text("It disappears from your saved sources. Installed apps stay.")
        }
        .onChange(of: model.errorMessage) { message in
            if let message { announce(message) }
        }
        .task {
            if model.contents == nil { await model.load() }
        }
    }

    private var apps: some View {
        List {
            // Errors sit above whatever has already loaded.
            if let message = browse.errorMessage {
                ErrorLabel(message: message)
                    .listRowBackground(OrbitColor.surface)
            }
            if let message = model.errorMessage {
                Group {
                    ErrorLabel(message: message)
                    Button("Retry") { Task { await model.load() } }
                }
                .listRowBackground(OrbitColor.surface)
            }
            if let contents = model.contents {
                ForEach(contents.apps) { app in
                    NavigationLink {
                        AppDetailView(app: app)
                    } label: {
                        VStack(alignment: .leading, spacing: OrbitSpace.xs) {
                            Text(app.name).foregroundStyle(OrbitColor.text)
                            if let developer = app.developer {
                                Text(developer)
                                    .font(.caption)
                                    .foregroundStyle(OrbitColor.textMuted)
                            }
                            if let version = app.version {
                                Text(version)
                                    .font(.footnote.monospaced())
                                    .foregroundStyle(OrbitColor.textMuted)
                            }
                        }
                    }
                    .listRowBackground(OrbitColor.surface)
                }
            } else if model.errorMessage == nil {
                ProgressView("Loading apps…")
                    .listRowBackground(OrbitColor.surface)
            }
        }
        .orbitList()
    }
}

struct AppDetailView: View {
    let app: AltSource.App

    var body: some View {
        List {
            Section {
                // The navigation title truncates at large text sizes.
                Text(app.name)
                    .font(.title2.weight(.semibold))
                    .accessibilityAddTraits(.isHeader)
                if let summary = app.summary {
                    Text(summary)
                        .frame(maxWidth: 680, alignment: .leading)
                }
            }
            .listRowBackground(OrbitColor.surface)

            Section {
                row("Developer", app.developer)
                row("Version", app.version, mono: true)
                row("Size", app.size.map { ByteCountFormatter.string(fromByteCount: Int64($0), countStyle: .file) })
                row("Needs iOS", app.minOSVersion)
                row("Bundle ID", app.bundleID, mono: true)
                row("Downloads from", app.downloadURL.host, mono: true)
            } header: {
                Text("Details").foregroundStyle(OrbitColor.textMuted)
            }
            .listRowBackground(OrbitColor.surface)

            Section {
                    if app.unavailableOnFreeAccounts.isEmpty && app.privacyKeys.isEmpty {
                        Text("This source doesn't list any permissions.")
                            .foregroundStyle(OrbitColor.textMuted)
                    }
                    ForEach(app.unavailableOnFreeAccounts, id: \.self) { name in
                        Label("\(name): not available on free accounts", systemImage: "exclamationmark.triangle.fill")
                            .foregroundStyle(OrbitColor.warning)
                    }
                    ForEach(app.privacyKeys, id: \.self) { key in
                        Label(Self.readable(key), systemImage: "hand.raised")
                    }
            } header: {
                Text("Permissions it asks for").foregroundStyle(OrbitColor.textMuted)
            }
            .listRowBackground(OrbitColor.surface)

            Section {
                Text("Installing apps isn't built yet.")
                    .foregroundStyle(OrbitColor.textMuted)
            }
            .listRowBackground(OrbitColor.surface)
        }
        .orbitList()
        .navigationTitle(app.name)
    }

    @ViewBuilder
    private func row(_ title: String, _ value: String?, mono: Bool = false) -> some View {
        if let value {
            LabeledContent(title) {
                Text(value)
                    .font(mono ? .footnote.monospaced() : .body)
                    .foregroundStyle(OrbitColor.textMuted)
                    .textSelection(.enabled)
            }
        }
    }

    private static let permissionNames = [
        "NSCameraUsageDescription": "Camera",
        "NSMicrophoneUsageDescription": "Microphone",
        "NSPhotoLibraryUsageDescription": "Your photos",
        "NSPhotoLibraryAddUsageDescription": "Saving to your photos",
        "NSLocationWhenInUseUsageDescription": "Your location while in use",
        "NSLocationAlwaysAndWhenInUseUsageDescription": "Your location at any time",
        "NSContactsUsageDescription": "Your contacts",
        "NSCalendarsUsageDescription": "Your calendar",
        "NSBluetoothAlwaysUsageDescription": "Bluetooth",
        "NSLocalNetworkUsageDescription": "Devices on your network",
        "NSFaceIDUsageDescription": "Face ID",
    ]

    /// "NSCameraUsageDescription" reads as "Camera".
    private static func readable(_ key: String) -> String {
        if let known = permissionNames[key] { return known }
        var name = key
        if name.hasPrefix("NS") { name.removeFirst(2) }
        return name.replacingOccurrences(of: "UsageDescription", with: "")
    }
}

#Preview("Dark") {
    NavigationStack {
        BrowseView(model: .sample)
    }
    .preferredColorScheme(.dark)
}

#Preview("Largest text") {
    NavigationStack {
        BrowseView(model: .sample)
    }
    .environment(\.dynamicTypeSize, .accessibility5)
}
