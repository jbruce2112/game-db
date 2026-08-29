import SwiftUI
import VisionKit

struct BarcodeScannerView: UIViewControllerRepresentable {
    var onCode: (String) -> Void

    static var isAvailable: Bool {
        DataScannerViewController.isSupported && DataScannerViewController.isAvailable
    }

    func makeCoordinator() -> Coordinator {
        Coordinator(onCode: onCode)
    }

    func makeUIViewController(context: Context) -> DataScannerViewController {
        let vc = DataScannerViewController(
            recognizedDataTypes: [.barcode(symbologies: [.ean8, .ean13, .upce])],
            qualityLevel: .balanced,
            recognizesMultipleItems: false,
            isHighlightingEnabled: true
        )
        vc.delegate = context.coordinator
        return vc
    }

    func updateUIViewController(_ vc: DataScannerViewController, context: Context) {
        context.coordinator.onCode = onCode
        // Do not restart after a successful read — SwiftUI calls update
        // when the sheet dismisses, which used to fire a second lookup and
        // wipe the selected game (no Add to Library button).
        guard !context.coordinator.handled else { return }
        if !vc.isScanning {
            try? vc.startScanning()
        }
    }

    final class Coordinator: NSObject, DataScannerViewControllerDelegate {
        var onCode: (String) -> Void
        var handled = false

        init(onCode: @escaping (String) -> Void) {
            self.onCode = onCode
        }

        func dataScanner(_ dataScanner: DataScannerViewController, didAdd addedItems: [RecognizedItem], allItems: [RecognizedItem]) {
            guard !handled else { return }
            for item in addedItems {
                if case .barcode(let code) = item, let value = code.payloadStringValue {
                    let digits = value.filter(\.isNumber)
                    // Box UPCs are 12 or 13 digits; ignore short/partial reads.
                    guard digits.count == 12 || digits.count == 13 else { continue }
                    handled = true
                    dataScanner.stopScanning()
                    DispatchQueue.main.async { self.onCode(digits) }
                    return
                }
            }
        }
    }
}
