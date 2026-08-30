import SwiftUI
import UIKit

struct LibraryView: View {
    @Environment(LibraryStore.self) private var store
    @State private var showAdd = false
    @State private var addTab = 0
    @State private var showSettings = false
    @State private var csvShare: CSVShare?
    @State private var compactColumn: NavigationSplitViewColumn = .detail
    @State private var pendingDelete: LibraryItem?
    @State private var openFromMenu: LibraryItem?

    var body: some View {
        NavigationSplitView(preferredCompactColumn: $compactColumn) {
            PlatformSidebar {
                compactColumn = .detail
            }
                .navigationTitle("Platforms")
                .toolbar {
                    ToolbarItem(placement: .topBarLeading) {
                        Button { showSettings = true } label: {
                            Image(systemName: store.isPaired ? "externaldrive.badge.checkmark" : "externaldrive")
                        }
                    }
                    ToolbarItem(placement: .topBarTrailing) {
                        Button("Library") { compactColumn = .detail }
                    }
                }
        } detail: {
            libraryStack
        }
        .navigationSplitViewColumnWidth(min: 200, ideal: 240, max: 320)
        .preferredColorScheme(.dark)
        .sheet(item: $csvShare) { share in
            ShareSheet(items: [share.url])
        }
        .sheet(isPresented: $showAdd) { AddGameView(initialTab: addTab) }
        .sheet(isPresented: $showSettings) { SettingsView() }
    }

    private var libraryStack: some View {
        NavigationStack {
            Group {
                if store.filtered.isEmpty {
                    ContentUnavailableView(
                        emptyTitle,
                        systemImage: "square.stack",
                        description: Text(emptyDescription)
                    )
                    .padding(.bottom, 88)
                } else {
                    ScrollView {
                        LazyVGrid(columns: [GridItem(.adaptive(minimum: 110), spacing: 12)], spacing: 12) {
                            ForEach(store.filtered) { item in
                                NavigationLink(value: item) {
                                    GameGridCell(item: item)
                                }
                                .buttonStyle(.plain)
                                .contextMenu {
                                    Button("View details", systemImage: "info.circle") {
                                        openFromMenu = item
                                    }
                                    Button("Delete", systemImage: "trash", role: .destructive) {
                                        pendingDelete = item
                                    }
                                } preview: {
                                    GamePeekPreview(
                                        item: item,
                                        image: store.cachedCoverImage(for: item.coverId)
                                    )
                                }
                            }
                        }
                        .padding()
                        .padding(.bottom, 72)
                    }
                    .refreshable { await store.runSync() }
                }
            }
            .navigationTitle(store.platformFilter.isEmpty ? "Library" : store.platformFilter)
            .navigationDestination(for: LibraryItem.self) { item in
                GameDetailView(itemID: item.id)
            }
            .navigationDestination(item: $openFromMenu) { item in
                GameDetailView(itemID: item.id)
            }
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button { showSettings = true } label: {
                        Image(systemName: store.isPaired ? "externaldrive.badge.checkmark" : "externaldrive")
                    }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Menu {
                        Button("Title") { store.sort = "title" }
                        Button("Date added") { store.sort = "added" }
                    } label: {
                        Image(systemName: "arrow.up.arrow.down")
                    }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        let active = store.items.filter { ($0.deletedAt ?? "").isEmpty }
                        if let url = try? LibraryCSV.fileURL(from: active) {
                            csvShare = CSVShare(url: url)
                        }
                    } label: {
                        Image(systemName: "square.and.arrow.up")
                    }
                    .disabled(store.items.filter { ($0.deletedAt ?? "").isEmpty }.isEmpty)
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        addTab = 1
                        showAdd = true
                    } label: { Image(systemName: "barcode.viewfinder") }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        addTab = 0
                        showAdd = true
                    } label: { Image(systemName: "plus") }
                }
            }
            .overlay(alignment: .bottom) {
                FloatingSearchBar()
                    .padding(.bottom, 10)
            }
            .confirmationDialog(
                "Delete \(pendingDelete?.title ?? "this game")?",
                isPresented: Binding(
                    get: { pendingDelete != nil },
                    set: { if !$0 { pendingDelete = nil } }
                ),
                titleVisibility: .visible
            ) {
                Button("Delete", role: .destructive) {
                    if let pendingDelete {
                        try? store.delete(pendingDelete)
                    }
                    pendingDelete = nil
                }
                Button("Cancel", role: .cancel) { pendingDelete = nil }
            }
        }
    }

    private var emptyTitle: String {
        if !store.query.isEmpty || !store.platformFilter.isEmpty {
            return "No matching games"
        }
        return "Nothing on the shelf yet"
    }

    private var emptyDescription: String {
        if !store.query.isEmpty || !store.platformFilter.isEmpty {
            return "Try a different title or clear the filter."
        }
        return "Add a game, even while offline."
    }
}

private struct GameGridCell: View {
    var item: LibraryItem

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            CoverView(item: item)
                .frame(maxWidth: .infinity)
                .aspectRatio(3/4, contentMode: .fit)
                .clipShape(RoundedRectangle(cornerRadius: 8))
            Text(item.title)
                .font(.caption)
                .foregroundStyle(.primary)
                .lineLimit(2)
            Text(item.platform)
                .font(.caption2)
                .foregroundStyle(.secondary)
                .lineLimit(1)
        }
    }
}

struct GamePeekPreview: View {
    var item: LibraryItem
    var image: UIImage?

    /// Context-menu previews are snapshotted in an isolated host with no
    /// proposed size. A flexible VStack collapsed to empty; pin 3:4 box art.
    var body: some View {
        ZStack {
            Color(white: 0.12)
            if let resolvedImage {
                Image(uiImage: resolvedImage)
                    .renderingMode(.original)
                    .resizable()
                    .scaledToFill()
            } else {
                Text(item.title)
                    .font(.headline)
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                    .padding(16)
            }
        }
        .frame(width: 240, height: 320)
        .clipped()
    }

    private var resolvedImage: UIImage? {
        image ?? CoverFile.image(for: item.coverId)
    }
}

private struct PlatformSidebar: View {
    @Environment(LibraryStore.self) private var store
    var onPick: () -> Void

    var body: some View {
        List {
            platformRow(label: "All games", count: store.shelfCount, id: "")
            ForEach(store.platformCounts, id: \.name) { row in
                platformRow(label: row.name, count: row.count, id: row.name)
            }
        }
        .listStyle(.sidebar)
    }

    private func platformRow(label: String, count: Int, id: String) -> some View {
        let selected = store.platformFilter == id
        return Button {
            store.platformFilter = id
            onPick()
        } label: {
            HStack {
                Text(label)
                    .foregroundStyle(selected ? Color(red: 0.89, green: 0.69, blue: 0.29) : .primary)
                    .lineLimit(1)
                Spacer()
                Text("\(count)")
                    .font(.subheadline.monospacedDigit())
                    .foregroundStyle(selected ? Color(red: 0.89, green: 0.69, blue: 0.29) : .secondary)
            }
        }
        .listRowBackground(selected ? Color(red: 0.89, green: 0.69, blue: 0.29).opacity(0.16) : Color.clear)
        .accessibilityAddTraits(selected ? .isSelected : [])
    }
}

private struct FloatingSearchBar: View {
    @Environment(LibraryStore.self) private var store
    @FocusState private var isFocused: Bool

    private var expanded: Bool { isFocused || !store.query.isEmpty }

    var body: some View {
        @Bindable var store = store
        HStack(spacing: 12) {
            HStack(spacing: 8) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(.secondary)
                TextField("Search", text: $store.query)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .submitLabel(.search)
                    .focused($isFocused)
                    .multilineTextAlignment(expanded ? .leading : .center)
                if expanded && !store.query.isEmpty {
                    Button {
                        store.query = ""
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundStyle(.secondary)
                    }
                    .accessibilityLabel("Clear search")
                }
                if !expanded {
                    statusDot
                }
            }
            .padding(.horizontal, expanded ? 14 : 16)
            .padding(.vertical, 11)
            .libraryGlassCapsule()
            .frame(maxWidth: expanded ? .infinity : 200)
            .contentShape(Capsule())

            if expanded {
                Button("Cancel") {
                    store.query = ""
                    isFocused = false
                }
                .font(.body)
                .foregroundStyle(.primary)
                .transition(.opacity.combined(with: .move(edge: .trailing)))
            }
        }
        .padding(.horizontal, expanded ? 16 : 0)
        .frame(maxWidth: .infinity)
        .animation(.spring(duration: 0.32, bounce: 0.12), value: expanded)
        .accessibilityElement(children: .contain)
    }

    private var statusDot: some View {
        Circle()
            .fill(statusColor)
            .frame(width: 7, height: 7)
            .opacity(0.55)
            .padding(.leading, 2)
            .accessibilityLabel(statusText)
    }

    private var statusColor: Color {
        if !store.isPaired { return .secondary }
        return store.online ? .green : .orange
    }

    private var statusText: String {
        if !store.isPaired { return "Local only" }
        if store.online {
            if let last = store.lastSync {
                return "Online, last synced \(last.formatted(date: .omitted, time: .shortened))"
            }
            return "Online"
        }
        return store.syncMessage.isEmpty ? "Offline" : store.syncMessage
    }
}

private extension View {
    @ViewBuilder
    func libraryGlassCapsule() -> some View {
        if #available(iOS 26.0, *) {
            self.glassEffect(.regular.interactive(), in: Capsule())
        } else {
            self
                .background(.ultraThinMaterial, in: Capsule())
                .overlay {
                    Capsule().strokeBorder(.white.opacity(0.14), lineWidth: 0.5)
                }
                .shadow(color: .black.opacity(0.28), radius: 18, y: 8)
        }
    }
}

private struct CSVShare: Identifiable {
    let id = UUID()
    let url: URL
}

private struct ShareSheet: UIViewControllerRepresentable {
    var items: [Any]

    func makeUIViewController(context: Context) -> UIActivityViewController {
        UIActivityViewController(activityItems: items, applicationActivities: nil)
    }

    func updateUIViewController(_ uiViewController: UIActivityViewController, context: Context) {}
}
