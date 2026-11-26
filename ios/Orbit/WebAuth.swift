import AuthenticationServices
import OrbitKit
import UIKit

struct SheetFailed: LocalizedError {
    var errorDescription: String? { "The sign-in page didn't open. Close other pop-ups and try again." }
}

/// Runs the provider's sign-in page in the system sheet.
@MainActor
final class WebAuth: NSObject, WebAuthenticator, ASWebAuthenticationPresentationContextProviding {
    private var session: ASWebAuthenticationSession?

    func authenticate(url: URL, callbackScheme: String) async throws -> URL {
        defer { session = nil }
        return try await withCheckedThrowingContinuation { continuation in
            let session = ASWebAuthenticationSession(url: url, callbackURLScheme: callbackScheme) { callback, error in
                if let callback {
                    continuation.resume(returning: callback)
                } else if (error as? ASWebAuthenticationSessionError)?.code == .canceledLogin {
                    continuation.resume(throwing: SignInCancelled())
                } else {
                    continuation.resume(throwing: SheetFailed())
                }
            }
            session.presentationContextProvider = self
            self.session = session
            if !session.start() {
                continuation.resume(throwing: SheetFailed())
            }
        }
    }

    func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap(\.windows)
            .first(where: \.isKeyWindow) ?? ASPresentationAnchor()
    }
}
