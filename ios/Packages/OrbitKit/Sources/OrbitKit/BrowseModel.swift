import Combine
import Foundation

private func describe(_ error: Error) -> String {
    (error as? LocalizedError)?.errorDescription ?? "Something went wrong. Try again."
}

/// The saved sources, and adding or removing one.
@MainActor
public final class BrowseModel: ObservableObject {
    /// nil until loaded.
    @Published public private(set) var sources: [SavedSource]?
    @Published public private(set) var errorMessage: String?
    @Published public var address = "" {
        didSet {
            // A preview belongs to the address it was made from.
            if address != oldValue { preview = nil }
        }
    }
    @Published public private(set) var preview: SourcePreview?
    @Published public private(set) var addError: String?
    @Published public private(set) var busy = false

    private let client: @MainActor () -> APIClient?
    private let sessionEnded: @MainActor () async -> Void

    public init(client: @escaping @MainActor () -> APIClient?, sessionEnded: @escaping @MainActor () async -> Void) {
        self.client = client
        self.sessionEnded = sessionEnded
    }

    public func load() async {
        errorMessage = nil
        do {
            if let list = try await client()?.sources() { sources = list }
        } catch {
            errorMessage = await explain(error)
        }
    }

    public func check() async {
        addError = nil
        preview = nil
        let entered = address.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let url = URL(string: entered), url.scheme?.lowercased() == "https", url.host?.contains(".") == true else {
            addError = "Enter a full address that starts with https://"
            return
        }
        await work {
            let found = try await self.client()?.previewSource(entered)
            // Ignore an answer for an address that has since been edited.
            if self.address.trimmingCharacters(in: .whitespacesAndNewlines) == entered { self.preview = found }
        }
    }

    /// Saves the previewed source. Returns true when it was saved.
    public func save() async -> Bool {
        guard let preview else { return false }
        addError = nil
        var saved = false
        await work {
            try await self.client()?.addSource(preview.url)
            saved = true
        }
        if saved {
            address = ""
            await load()
        }
        return saved
    }

    public func remove(_ source: SavedSource) async {
        errorMessage = nil
        do {
            try await client()?.deleteSource(id: source.id)
            sources?.removeAll { $0.id == source.id }
        } catch {
            errorMessage = await explain(error)
        }
    }

    private func work(_ job: @MainActor () async throws -> Void) async {
        guard !busy else { return }
        busy = true
        defer { busy = false }
        do {
            try await job()
        } catch {
            addError = await explain(error)
        }
    }

    private func explain(_ error: Error) async -> String {
        if error as? APIError == .signedOut { await sessionEnded() }
        return describe(error)
    }
}

/// The apps inside one source.
@MainActor
public final class SourceModel: ObservableObject {
    public let source: SavedSource
    @Published public private(set) var contents: AltSource?
    @Published public private(set) var errorMessage: String?
    private let fetcher: SourceFetcher

    public init(source: SavedSource, fetcher: SourceFetcher = SourceFetcher()) {
        self.source = source
        self.fetcher = fetcher
    }

    public func load() async {
        errorMessage = nil
        guard let url = URL(string: source.url) else {
            errorMessage = describe(AltSource.Invalid())
            return
        }
        do {
            contents = try await fetcher.fetch(url)
        } catch {
            errorMessage = describe(error)
        }
    }
}
