import Combine
import Foundation

/// Keeps this iPhone listed on the user's dashboard.
@MainActor
public final class DeviceModel: ObservableObject {
    public enum State: Equatable {
        case unknown
        case listed(name: String)
        /// The user removed it on the dashboard. It isn't re-added unasked.
        case removed
        case failed(String)
    }

    @Published public private(set) var state = State.unknown

    private let client: @MainActor () -> APIClient?
    private let registration: @MainActor () throws -> DeviceRegistration
    private let defaults: UserDefaults
    private let now: () -> Date
    private var running = false

    private static let idKey = "orbit.device.id"
    private static let nameKey = "orbit.device.name"
    private static let removedKey = "orbit.device.removed"
    private static let reportedKey = "orbit.device.reported"
    /// docs/IOS_APP.md: at most one report on launch every ten minutes.
    private static let interval: TimeInterval = 600

    public init(
        defaults: UserDefaults = .standard, now: @escaping () -> Date = Date.init,
        client: @escaping @MainActor () -> APIClient?,
        registration: @escaping @MainActor () throws -> DeviceRegistration
    ) {
        self.defaults = defaults
        self.now = now
        self.client = client
        self.registration = registration
    }

    /// Call on launch and when the app returns: registers once, then checks in.
    public func checkIn() async {
        guard !running, let client = client() else { return }
        if defaults.bool(forKey: Self.removedKey) {
            state = .removed
            return
        }
        running = true
        defer { running = false }
        do {
            guard let id = defaults.string(forKey: Self.idKey) else {
                try await register(client)
                return
            }
            let name = defaults.string(forKey: Self.nameKey) ?? "This iPhone"
            let last = defaults.object(forKey: Self.reportedKey) as? Date
            if last.map({ now().timeIntervalSince($0) >= Self.interval }) ?? true {
                try await client.report(DeviceReport(reportedAt: now()), deviceID: id)
                defaults.set(now(), forKey: Self.reportedKey)
            }
            state = .listed(name: name)
        } catch APIError.server(status: 404, _, _, _) {
            defaults.removeObject(forKey: Self.idKey)
            defaults.set(true, forKey: Self.removedKey)
            state = .removed
        } catch {
            state = .failed((error as? LocalizedError)?.errorDescription ?? "Couldn't update the dashboard.")
        }
    }

    /// The user asked to list a removed iPhone again.
    public func addAgain() async {
        defaults.removeObject(forKey: Self.removedKey)
        await checkIn()
    }

    /// On sign-out: the next account registers afresh.
    public func forget() {
        for key in [Self.idKey, Self.nameKey, Self.removedKey, Self.reportedKey] {
            defaults.removeObject(forKey: key)
        }
        state = .unknown
    }

    private func register(_ client: APIClient) async throws {
        let device = try await client.registerDevice(registration())
        defaults.set(device.id, forKey: Self.idKey)
        defaults.set(device.name, forKey: Self.nameKey)
        defaults.set(now(), forKey: Self.reportedKey)
        state = .listed(name: device.name)
    }
}
