import SwiftUI

struct LibraryView: View {
    @Environment(LibraryStore.self) private var store
    @State private var showAdd = false
    @State private var showSettings = false

    var body: some View {
        @Bindable var store = store
        NavigationStack {
            Group {
                if store.filtered.isEmpty {
                    ContentUnavailableView(
                        "Nothing on the shelf yet",
                        systemImage: "square.stack",
                        description: Text("Add a game, even while offline.")
                    )
                } else {
                    ScrollView {
                        LazyVGrid(columns: [GridItem(.adaptive(minimum: 110), spacing: 12)], spacing: 12) {
                            ForEach(store.filtered) { item in
                                NavigationLink(value: item) {
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
                                .buttonStyle(.plain)
                            }
                        }
                        .padding()
                    }
                    .refreshable { await store.runSync() }
                }
            }
            .navigationTitle("Library")
            .navigationDestination(for: LibraryItem.self) { item in
                GameDetailView(itemID: item.id)
            }
            .searchable(text: $store.query, prompt: "Titles")
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
                        Divider()
                        Button("All platforms") { store.platformFilter = "" }
                        ForEach(store.platforms, id: \.self) { p in
                            Button(p) { store.platformFilter = p }
                        }
                    } label: {
                        Image(systemName: "line.3.horizontal.decrease.circle")
                    }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button { showAdd = true } label: { Image(systemName: "plus") }
                }
            }
            .safeAreaInset(edge: .bottom) {
                HStack {
                    Circle()
                        .fill(store.online ? Color.green : Color.secondary)
                        .frame(width: 8, height: 8)
                    Text(statusText)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Spacer()
                    if !store.platformFilter.isEmpty {
                        Button("Clear filter") { store.platformFilter = "" }
                            .font(.caption)
                    }
                }
                .padding(.horizontal)
                .padding(.vertical, 8)
                .background(.thinMaterial)
            }
            .sheet(isPresented: $showAdd) { AddGameView() }
            .sheet(isPresented: $showSettings) { SettingsView() }
        }
        .preferredColorScheme(.dark)
    }

    private var statusText: String {
        if !store.isPaired { return "Local only" }
        if store.online {
            if let last = store.lastSync {
                return "Synced \(last.formatted(date: .omitted, time: .shortened))"
            }
            return "Online"
        }
        return store.syncMessage.isEmpty ? "Offline" : store.syncMessage
    }
}
