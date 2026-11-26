import SwiftUI

/// The main action on a screen.
struct OrbitPrimaryStyle: ButtonStyle {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.headline)
            .multilineTextAlignment(.center)
            .foregroundStyle(OrbitColor.onPrimary)
            .padding(.horizontal, OrbitSpace.l)
            .padding(.vertical, OrbitSpace.m)
            .frame(maxWidth: .infinity, minHeight: 44)
            .background(OrbitColor.primary, in: RoundedRectangle(cornerRadius: OrbitRadius.md))
            .opacity(isEnabled ? (configuration.isPressed ? 0.7 : 1) : 0.5)
            .scaleEffect(configuration.isPressed && !reduceMotion ? 0.97 : 1)
            .animation(OrbitMotion.instant, value: configuration.isPressed)
    }
}

/// A secondary action, outlined.
struct OrbitSecondaryStyle: ButtonStyle {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.headline)
            .multilineTextAlignment(.center)
            .foregroundStyle(OrbitColor.text)
            .padding(.horizontal, OrbitSpace.l)
            .padding(.vertical, OrbitSpace.m)
            .frame(maxWidth: .infinity, minHeight: 44)
            .background(OrbitColor.surface, in: RoundedRectangle(cornerRadius: OrbitRadius.md))
            .overlay(RoundedRectangle(cornerRadius: OrbitRadius.md).strokeBorder(OrbitColor.borderStrong))
            .opacity(isEnabled ? (configuration.isPressed ? 0.7 : 1) : 0.5)
            .scaleEffect(configuration.isPressed && !reduceMotion ? 0.97 : 1)
            .animation(OrbitMotion.instant, value: configuration.isPressed)
    }
}

/// Every list has one of these when empty: a symbol, a sentence and an action.
struct EmptyStateView<Action: View>: View {
    let symbol: String
    let title: String
    let message: String
    @ViewBuilder let action: Action

    var body: some View {
        ScrollView {
            VStack(spacing: OrbitSpace.l) {
                Image(systemName: symbol)
                    .symbolRenderingMode(.hierarchical)
                    .font(.largeTitle)
                    .foregroundStyle(OrbitColor.textMuted)
                    .accessibilityHidden(true)
                Text(title)
                    .font(.title2.weight(.semibold))
                    .foregroundStyle(OrbitColor.text)
                    .accessibilityAddTraits(.isHeader)
                Text(message)
                    .foregroundStyle(OrbitColor.textMuted)
                action
            }
            .multilineTextAlignment(.center)
            .frame(maxWidth: 480)
            .frame(maxWidth: .infinity)
            .padding(.horizontal, OrbitSpace.l)
            .padding(.vertical, OrbitSpace.section)
        }
        .background(OrbitColor.bg.ignoresSafeArea())
    }
}

/// An error with an icon, so colour isn't the only signal.
struct ErrorLabel: View {
    let message: String

    var body: some View {
        Label(message, systemImage: "exclamationmark.triangle.fill")
            .font(.callout)
            .foregroundStyle(OrbitColor.danger)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

extension View {
    /// A card on the page background.
    func orbitCard() -> some View {
        frame(maxWidth: .infinity, alignment: .leading)
            .padding(OrbitSpace.l)
            .background(OrbitColor.surface, in: RoundedRectangle(cornerRadius: OrbitRadius.lg))
    }

    /// Token colours for a `List` or `Form`.
    func orbitList() -> some View {
        scrollContentBackground(.hidden)
            .background(OrbitColor.bg.ignoresSafeArea())
    }
}

/// Status as icon, words and colour together.
struct StatusLabel: View {
    let status: FreshnessStatus

    var body: some View {
        Label(status.label, systemImage: status.symbol)
            .font(.title2.weight(.semibold))
            .monospacedDigit()
            .foregroundStyle(status.color)
    }
}
