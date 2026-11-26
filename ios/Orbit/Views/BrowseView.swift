import OrbitKit
import SwiftUI

struct BrowseView: View {
    @ObservedObject var model: BrowseModel
    @State private var adding = false
    @State private var removing: SavedSource?

    var body: some View {
        content
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
                    ErrorLabel(message: message)
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
                        Button("Remove", role: .destructive) { removing = source }
                    }
                }
            }
            .orbitList()
            .refreshable { await model.load() }
        } else if model.sources != nil {
            EmptyStateView(symbol: "square.grid.2x2", title: "No sources yet", message: "Add a link to an app source to see its apps.") {
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
                        Text("\(preview.appCount) apps")
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
        List {
            if let message = model.errorMessage {
                ErrorLabel(message: message)
                Button("Retry") { Task { await model.load() } }
            } else if let contents = model.contents {
                if contents.apps.isEmpty {
                    Text("This source lists no apps Orbit can read.")
                        .foregroundStyle(OrbitColor.textMuted)
                }
                ForEach(contents.apps) { app in
                    NavigationLink {
                        AppDetailView(app: app)
                    } label: {
                        VStack(alignment: .leading, spacing: OrbitSpace.xs) {
                            Text(app.name).foregroundStyle(OrbitColor.text)
                            Text([app.developer, app.version].compactMap { $0 }.joined(separator: " · "))
                                .font(.caption)
                                .foregroundStyle(OrbitColor.textMuted)
                        }
                    }
                }
            } else {
                ProgressView("Loading apps…")
            }
        }
        .orbitList()
        .navigationTitle(model.source.title)
        .toolbar {
            Button {
                confirming = true
            } label: {
                Label("Remove source", systemImage: "trash")
            }
        }
        .confirmationDialog("Remove \(model.source.title)?", isPresented: $confirming, titleVisibility: .visible) {
            Button("Remove \(model.source.title)", role: .destructive) {
                Task {
                    await browse.remove(model.source)
                    dismiss()
                }
            }
        }
        .task { await model.load() }
    }
}

struct AppDetailView: View {
    let app: AltSource.App

    var body: some View {
        List {
            if let summary = app.summary {
                Section {
                    Text(summary)
                }
                .listRowBackground(OrbitColor.surface)
            }

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

            if !app.unavailableOnFreeAccounts.isEmpty || !app.privacyKeys.isEmpty {
                Section {
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
            }

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

    /// "NSCameraUsageDescription" reads as "Camera".
    private static func readable(_ key: String) -> String {
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
