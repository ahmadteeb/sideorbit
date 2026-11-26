import OrbitKit
import SwiftUI
import UserNotifications

struct HomeView: View {
    @ObservedObject var device: DeviceModel
    let server: URL?
    var expiry: Date? = ThisDevice.orbitExpiry()

    @Environment(\.openURL) private var openURL
    @State private var reminders = Reminders.wanted
    @State private var blocked = false

    var body: some View {
        ScrollView {
            VStack(spacing: OrbitSpace.xl) {
                orbit
                listing
                Text("Installing and refreshing other apps isn't built yet.")
                    .font(.callout)
                    .foregroundStyle(OrbitColor.textMuted)
                    .multilineTextAlignment(.center)
            }
            .frame(maxWidth: 680)
            .frame(maxWidth: .infinity)
            .padding(OrbitSpace.l)
        }
        .background(OrbitColor.bg.ignoresSafeArea())
        .navigationTitle("Home")
        .task {
            blocked = await Reminders.blocked()
            if reminders, let expiry { await Reminders.schedule(expiry: expiry) }
        }
    }

    private var orbit: some View {
        VStack(alignment: .leading, spacing: OrbitSpace.m) {
            if let expiry {
                // Countdowns move on their own, once a minute.
                TimelineView(.everyMinute) { context in
                    StatusLabel(status: status(expiry, now: context.date))
                }
                Text("Orbit itself stops opening after \(expiry.formatted(date: .abbreviated, time: .shortened)).")
                    .foregroundStyle(OrbitColor.textMuted)
                reminderControls(expiry)
            } else {
                Label("Orbit's expiry isn't known", systemImage: "questionmark.circle")
                    .font(.title2.weight(.semibold))
                    .foregroundStyle(OrbitColor.textMuted)
                Text("This copy has no provisioning profile to read.")
                    .foregroundStyle(OrbitColor.textMuted)
            }
        }
        .orbitCard()
    }

    @ViewBuilder
    private func reminderControls(_ expiry: Date) -> some View {
        if blocked {
            Text("Notifications are off for Orbit. Allow them in Settings to be warned.")
                .foregroundStyle(OrbitColor.text)
            Button("Open Settings") {
                if let url = URL(string: UIApplication.openSettingsURLString) { openURL(url) }
            }
            .buttonStyle(OrbitSecondaryStyle())
        } else if Expiry.reminders(for: expiry, now: Date()).isEmpty {
            Text("It's too close to expiry to schedule a reminder. Install Orbit again now.")
                .foregroundStyle(OrbitColor.text)
        } else if reminders {
            Label("You'll be warned 48, 24 and 6 hours before.", systemImage: "bell")
                .foregroundStyle(OrbitColor.text)
            Button("Turn off reminders") {
                Reminders.disable()
                reminders = false
            }
            .buttonStyle(OrbitSecondaryStyle())
        } else {
            Button("Warn me before it expires") {
                Task {
                    reminders = await Reminders.enable(expiry: expiry)
                    blocked = await Reminders.blocked()
                }
            }
            .buttonStyle(OrbitPrimaryStyle())
        }
    }

    private var listing: some View {
        VStack(alignment: .leading, spacing: OrbitSpace.m) {
            switch device.state {
            case .unknown:
                ProgressView("Adding this iPhone to your dashboard…")
            case .listed(let name):
                Label("\(name) is on your dashboard", systemImage: "checkmark.circle")
                    .foregroundStyle(OrbitColor.text)
                if let server {
                    Link("Open the web dashboard", destination: server)
                        .buttonStyle(OrbitSecondaryStyle())
                }
            case .removed:
                Label("This iPhone was removed on the dashboard", systemImage: "minus.circle")
                    .foregroundStyle(OrbitColor.warning)
                Button("Add it again") { Task { await device.addAgain() } }
                    .buttonStyle(OrbitSecondaryStyle())
            case .failed(let message):
                ErrorLabel(message: message)
                Button("Retry") { Task { await device.checkIn() } }
                    .buttonStyle(OrbitSecondaryStyle())
            }
        }
        .orbitCard()
    }

    private func status(_ expiry: Date, now: Date) -> FreshnessStatus {
        switch Expiry.status(expiry, now: now) {
        case .fresh(let days): .fresh(daysLeft: days)
        case .expiring(let hours): .expiring(hoursLeft: hours)
        case .expired: .expired
        }
    }
}

#Preview("Fresh, dark") {
    NavigationStack {
        HomeView(device: .preview, server: nil, expiry: Date().addingTimeInterval(5 * 86400))
    }
    .preferredColorScheme(.dark)
}

#Preview("Expiring, largest text") {
    NavigationStack {
        HomeView(device: .preview, server: nil, expiry: Date().addingTimeInterval(20 * 3600))
    }
    .environment(\.dynamicTypeSize, .accessibility5)
}
