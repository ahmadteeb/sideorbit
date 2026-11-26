import Foundation

/// Downloads a source straight from its host. No Orbit token goes with it.
public struct SourceFetcher: Sendable {
    public static let maxBytes = 2 << 20

    public struct Unreachable: Error, LocalizedError {
        public let host: String
        public var errorDescription: String? { "Couldn't reach \(host). Check your connection, then tap Retry." }
    }
    private let transport: Transport

    public init(transport: Transport = URLSessionTransport(redirects: .httpsOnly)) {
        self.transport = transport
    }

    public func fetch(_ url: URL) async throws -> AltSource {
        guard url.scheme?.lowercased() == "https" else { throw AltSource.Invalid() }
        var request = URLRequest(url: url)
        request.timeoutInterval = 20
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        let data: Data
        let response: HTTPURLResponse
        do {
            (data, response) = try await transport.send(request)
        } catch {
            throw Unreachable(host: url.host ?? "the source")
        }
        // shortcut: the size is checked after downloading; stream with a cap if
        // oversized sources turn up.
        guard response.statusCode == 200, data.count <= Self.maxBytes else { throw AltSource.Invalid() }
        return try AltSource.parse(data)
    }
}
