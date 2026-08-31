import SwiftUI
import UIKit

struct CoverView: View {
    var item: LibraryItem
    @Environment(LibraryStore.self) private var store
    @State private var image: UIImage?

    private var boxMode: Bool { store.coverArt == .box }

    var body: some View {
        Group {
            if let image {
                Image(uiImage: image)
                    .resizable()
                    .interpolation(.high)
                    .aspectRatio(
                        boxMode ? image.size.width / max(image.size.height, 1) : 3 / 4,
                        contentMode: boxMode ? .fit : .fill
                    )
            } else {
                ZStack {
                    Color(white: 0.12)
                    Text(item.title)
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .multilineTextAlignment(.center)
                        .padding(6)
                }
                .aspectRatio(3 / 4, contentMode: .fit)
            }
        }
        .clipped()
        .clipShape(RoundedRectangle(cornerRadius: 8))
        .task(id: "\(item.id)-\(store.coverArt.rawValue)-\(item.coverId ?? "")-\(item.boxCoverId ?? "")") {
            image = await store.displayCover(for: item)
        }
    }
}
