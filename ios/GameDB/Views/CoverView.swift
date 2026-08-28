import SwiftUI
import UIKit

struct CoverView: View {
    var item: LibraryItem
    @Environment(LibraryStore.self) private var store
    @State private var image: UIImage?

    var body: some View {
        ZStack {
            Color(white: 0.12)
            if let image {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFill()
            } else {
                Text(item.title)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                    .padding(6)
            }
        }
        .clipped()
        .task(id: item.coverId) {
            image = await store.coverImage(for: item.coverId)
        }
    }
}
