import Foundation
import OrbitKit
import UserNotifications

/// Local notifications before Orbit's own signature expires.
enum Reminders {
    private static let wantedKey = "orbit.reminders"
    private static var center: UNUserNotificationCenter { .current() }

    static var wanted: Bool { UserDefaults.standard.bool(forKey: wantedKey) }

    /// Asks for permission (only from a tap) and schedules. False if refused.
    static func enable(expiry: Date) async -> Bool {
        let granted = (try? await center.requestAuthorization(options: [.alert, .sound])) ?? false
        UserDefaults.standard.set(granted, forKey: wantedKey)
        if granted { await schedule(expiry: expiry) }
        return granted
    }

    /// Replaces the schedule. A reinstall moves the expiry, so call this on launch.
    static func schedule(expiry: Date) async {
        center.removePendingNotificationRequests(withIdentifiers: Expiry.reminderIDs)
        for reminder in Expiry.reminders(for: expiry, now: Date()) {
            let content = UNMutableNotificationContent()
            content.title = reminder.title
            content.body = reminder.body
            content.sound = .default
            let trigger = UNTimeIntervalNotificationTrigger(
                timeInterval: max(1, reminder.date.timeIntervalSinceNow), repeats: false)
            try? await center.add(UNNotificationRequest(identifier: reminder.id, content: content, trigger: trigger))
        }
    }

    static func disable() {
        UserDefaults.standard.set(false, forKey: wantedKey)
        center.removePendingNotificationRequests(withIdentifiers: Expiry.reminderIDs)
    }

    /// Whether the user has since turned notifications off in Settings.
    static func blocked() async -> Bool {
        await center.notificationSettings().authorizationStatus == .denied
    }
}
