import Foundation

public enum Expiry {
    /// The expiry date in an app's `embedded.mobileprovision`. The file is a
    /// signed container around a property list.
    public static func date(inProvisioningProfile data: Data) -> Date? {
        guard let start = data.range(of: Data("<?xml".utf8)),
              let end = data.range(of: Data("</plist>".utf8), in: start.lowerBound..<data.endIndex),
              let plist = try? PropertyListSerialization.propertyList(
                from: data.subdata(in: start.lowerBound..<end.upperBound), format: nil) as? [String: Any]
        else { return nil }
        return plist["ExpirationDate"] as? Date
    }

    /// The status language of MASTER.md §3.
    public enum Status: Equatable, Sendable {
        case fresh(daysLeft: Int)
        case expiring(hoursLeft: Int)
        case expired
    }

    public static func status(_ expiry: Date, now: Date) -> Status {
        let left = expiry.timeIntervalSince(now)
        if left <= 0 { return .expired }
        if left <= 48 * 3600 { return .expiring(hoursLeft: Int(left / 3600)) }
        return .fresh(daysLeft: Int(left / 86400))
    }

    public struct Reminder: Equatable, Sendable {
        public let id: String
        public let date: Date
        public let title: String
        public let body: String
    }

    public static let reminderIDs = [48, 24, 6].map { "orbit-expiry-\($0)" }

    /// Warnings 48, 24 and 6 hours before Orbit itself expires.
    public static func reminders(for expiry: Date, now: Date) -> [Reminder] {
        [48, 24, 6].compactMap { hours in
            let date = expiry.addingTimeInterval(-Double(hours) * 3600)
            guard date > now else { return nil }
            return Reminder(
                id: "orbit-expiry-\(hours)", date: date,
                title: "Orbit expires in \(hours) hours",
                body: "Install it again from your computer before then, or it stops opening.")
        }
    }
}
