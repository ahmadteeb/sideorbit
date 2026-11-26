import SwiftUI

struct ContentView: View {
    var body: some View {
        ScrollView {
            VStack(spacing: OrbitSpace.l) {
                Image(systemName: "square.grid.2x2")
                    .symbolRenderingMode(.hierarchical)
                    .font(.largeTitle)
                    .foregroundStyle(OrbitColor.textMuted)
                    .accessibilityHidden(true)
                Text("Orbit")
                    .font(.largeTitle.weight(.semibold))
                    .foregroundStyle(OrbitColor.text)
                    .accessibilityAddTraits(.isHeader)
                Text("The app isn't built yet. It's coming in a later release.")
                    .font(.body)
                    .foregroundStyle(OrbitColor.textMuted)
                    .multilineTextAlignment(.center)
            }
            .frame(maxWidth: .infinity)
            .padding(.horizontal, OrbitSpace.l)
            .padding(.vertical, OrbitSpace.section)
        }
        .background(OrbitColor.bg.ignoresSafeArea())
    }
}
