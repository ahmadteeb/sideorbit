import OrbitKit
import UIKit

@MainActor
enum ThisDevice {
    static func registration() throws -> DeviceRegistration {
        var info = utsname()
        uname(&info)
        let model = withUnsafeBytes(of: info.machine) { String(decoding: $0.prefix(while: { $0 != 0 }), as: UTF8.self) }
        return DeviceRegistration(
            name: UIDevice.current.name, model: model, iosVersion: UIDevice.current.systemVersion,
            clientID: try InstallationID.loadOrCreate())
    }

    /// When this copy of Orbit stops opening, if it was signed with a profile.
    static func orbitExpiry() -> Date? {
        guard let url = Bundle.main.url(forResource: "embedded", withExtension: "mobileprovision"),
              let data = try? Data(contentsOf: url)
        else { return nil }
        return Expiry.date(inProvisioningProfile: data)
    }
}
