import Foundation

/// The self-hosted server this app talks to.
public enum ServerAddress {
    /// Accepts "orbit.example.com" or a full URL. Only HTTPS is allowed, because
    /// tokens travel with every request.
    public static func parse(_ text: String) -> URL? {
        var value = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !value.isEmpty else { return nil }
        if !value.contains("://") { value = "https://" + value }
        guard let parts = URLComponents(string: value),
              parts.scheme?.lowercased() == "https",
              let host = parts.host, host.contains("."), !host.contains(" "),
              parts.user == nil, parts.password == nil,
              parts.query == nil, parts.fragment == nil,
              parts.path.isEmpty || parts.path == "/"
        else { return nil }
        var clean = URLComponents()
        clean.scheme = "https"
        clean.host = host.lowercased()
        clean.port = parts.port
        return clean.url
    }
}
