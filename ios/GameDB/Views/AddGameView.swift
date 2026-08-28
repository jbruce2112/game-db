import SwiftUI

struct AddGameView: View {
    @Environment(LibraryStore.self) private var store
    @Environment(\.dismiss) private var dismiss
    @State private var tab = 0
    @State private var title = ""
    @State private var platform = ""
    @State private var region = "us"
    @State private var completeness = "unknown"
    @State private var notes = ""
    @State private var search = ""
    @State private var results: [SearchGame] = []
    @State private var picked: SearchGame?
    @State private var pickPlatform: Int64 = 0
    @State private var searching = false
    @State private var error: String?

    var body: some View {
        NavigationStack {
            Form {
                Picker("Mode", selection: $tab) {
                    Text("Search").tag(0)
                    Text("Manual").tag(1)
                }
                .pickerStyle(.segmented)

                if tab == 0 {
                    searchSection
                } else {
                    manualSection
                }
                if let error {
                    Section { Text(error).foregroundStyle(.red) }
                }
            }
            .navigationTitle("Add game")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                }
            }
        }
    }

    @ViewBuilder
    private var searchSection: some View {
        if !store.isPaired || !store.igdbConfigured {
            Section {
                Text("IGDB search needs a reachable server with IGDB keys. Use Manual, or pair in Settings.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        Section {
            HStack {
                TextField("Title", text: $search)
                Button("Go") { Task { await runSearch() } }
                    .disabled(search.trimmingCharacters(in: .whitespaces).isEmpty || searching)
            }
        }
        if let picked {
            Section(picked.name) {
                Picker("Platform", selection: $pickPlatform) {
                    ForEach(picked.platforms) { p in
                        Text(p.name).tag(p.id)
                    }
                }
                regionCompleteness
                Button("Add to library") { Task { await addFromIGDB() } }
            }
        } else {
            Section {
                ForEach(results) { game in
                    Button {
                        picked = game
                        pickPlatform = game.platforms.first?.id ?? 0
                    } label: {
                        VStack(alignment: .leading) {
                            Text(game.name)
                            Text(game.platforms.map(\.name).joined(separator: ", "))
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                }
            }
        }
    }

    private var manualSection: some View {
        Section {
            TextField("Title", text: $title)
            TextField("Platform", text: $platform)
            regionCompleteness
            TextField("Notes", text: $notes, axis: .vertical)
            Button("Add to library") { addManual() }
                .disabled(title.trimmingCharacters(in: .whitespaces).isEmpty || platform.trimmingCharacters(in: .whitespaces).isEmpty)
        }
    }

    private var regionCompleteness: some View {
        Group {
            Picker("Region", selection: $region) {
                Text("—").tag("")
                Text("US").tag("us")
                Text("EU").tag("eu")
                Text("JP").tag("jp")
                Text("AU").tag("au")
                Text("Other").tag("other")
            }
            Picker("Completeness", selection: $completeness) {
                Text("Unknown").tag("unknown")
                Text("Loose").tag("loose")
                Text("CIB").tag("cib")
                Text("New / sealed").tag("new")
            }
        }
    }

    private func runSearch() async {
        error = nil
        searching = true
        defer { searching = false }
        do {
            results = try await store.api.search(q: search)
            picked = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func addManual() {
        let item = LibraryItem.newLocal(
            title: title,
            platform: platform,
            region: region.isEmpty ? nil : region,
            completeness: completeness,
            notes: notes
        )
        try? store.add(item)
        dismiss()
    }

    private func addFromIGDB() async {
        guard let picked else { return }
        do {
            var item = try await store.api.createFromIGDB(
                gameId: picked.igdbId,
                platformId: pickPlatform,
                region: region.isEmpty ? nil : region,
                completeness: completeness
            )
            item.dirty = false
            try store.add(item)
            dismiss()
        } catch {
            self.error = error.localizedDescription
        }
    }
}
