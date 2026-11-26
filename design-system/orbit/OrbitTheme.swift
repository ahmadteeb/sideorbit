// OrbitTheme.swift: SwiftUI mapping of design-system/orbit/tokens.json.
// Copy into ios/Orbit/DesignSystem/ in Phase 0. Never use raw colors in views; use these.
import SwiftUI

extension Color {
    init(hex: UInt32) {
        self.init(.sRGB,
                  red: Double((hex >> 16) & 0xFF) / 255,
                  green: Double((hex >> 8) & 0xFF) / 255,
                  blue: Double(hex & 0xFF) / 255,
                  opacity: 1)
    }
    /// A color that resolves differently in light and dark mode.
    static func orbit(light: UInt32, dark: UInt32) -> Color {
        Color(UIColor { traits in
            let hex = traits.userInterfaceStyle == .dark ? dark : light
            return UIColor(Color(hex: hex))
        })
    }
}

enum OrbitColor {
    static let bg           = Color.orbit(light: 0xF8FAFC, dark: 0x0B1120)
    static let surface      = Color.orbit(light: 0xFFFFFF, dark: 0x151C2E)
    static let surface2     = Color.orbit(light: 0xF1F5F9, dark: 0x1E2740)
    static let text         = Color.orbit(light: 0x0F172A, dark: 0xF8FAFC)
    static let textMuted    = Color.orbit(light: 0x475569, dark: 0xA3AFC2)
    static let border       = Color.orbit(light: 0xE2E8F0, dark: 0x2A3550)
    static let borderStrong = Color.orbit(light: 0x64748B, dark: 0x64748B)
    static let primary      = Color.orbit(light: 0x15803D, dark: 0x22C55E)
    static let onPrimary    = Color.orbit(light: 0xFFFFFF, dark: 0x0B1120)
    static let warning      = Color.orbit(light: 0xB45309, dark: 0xFBBF24)
    static let danger       = Color.orbit(light: 0xB91C1C, dark: 0xF87171)
    static let info         = Color.orbit(light: 0x1D4ED8, dark: 0x60A5FA)
}

enum OrbitSpace {
    static let xs: CGFloat = 4, s: CGFloat = 8, m: CGFloat = 12, l: CGFloat = 16
    static let xl: CGFloat = 24, xxl: CGFloat = 32, section: CGFloat = 48
}

enum OrbitRadius {
    static let sm: CGFloat = 8, md: CGFloat = 12, lg: CGFloat = 16, xl: CGFloat = 24
}

enum OrbitMotion {
    static let instant = Animation.easeOut(duration: 0.10)
    static let fast    = Animation.easeOut(duration: 0.15)
    static let base    = Animation.easeOut(duration: 0.22)
    static let sheet   = Animation.spring(response: 0.35, dampingFraction: 0.85)
}

/// App freshness status. Always rendered as icon + word + color (never color alone).
enum FreshnessStatus {
    case fresh(daysLeft: Int), expiring(hoursLeft: Int), expired, refreshing, needsAction(String)

    var color: Color {
        switch self {
        case .fresh: OrbitColor.primary
        case .expiring, .needsAction: OrbitColor.warning
        case .expired: OrbitColor.danger
        case .refreshing: OrbitColor.info
        }
    }
    var symbol: String {
        switch self {
        case .fresh: "checkmark.seal.fill"
        case .expiring: "clock.badge.exclamationmark"
        case .expired: "xmark.seal.fill"
        case .refreshing: "arrow.triangle.2.circlepath"
        case .needsAction: "exclamationmark.triangle.fill"
        }
    }
    var label: String {
        switch self {
        case .fresh(let d): "Fresh · \(d)d left"
        case .expiring(let h): "Expires in \(h)h"
        case .expired: "Expired"
        case .refreshing: "Refreshing…"
        case .needsAction(let what): what
        }
    }
}

/// Press feedback: opacity + slight scale, no layout shift, respects Reduce Motion.
struct OrbitPressStyle: ButtonStyle {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .contentShape(Rectangle())
            .frame(minWidth: 44, minHeight: 44)
            .opacity(configuration.isPressed ? 0.7 : 1)
            .scaleEffect(configuration.isPressed && !reduceMotion ? 0.97 : 1)
            .animation(OrbitMotion.instant, value: configuration.isPressed)
    }
}
