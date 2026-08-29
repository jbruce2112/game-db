import SwiftUI

struct AddGameView: View {
    @Environment(LibraryStore.self) private var store
    @Environment(\.dismiss) private var dismiss
    var initialTab: Int = 0
    @State private var tab = 0
    @State private var title = ""
    @State private var platform = ""
    @State private var region = "us"
    @State private var completeness = "unknown"
    @State private var notes = ""
    @State private var barcode = ""
    @State private var search = ""
    @State private var results: [SearchGame] = []
    @State private var picked: SearchGame?
    @State private var pickPlatform: String = ""
    @State private var searching = false
    @State private var error: String?
    @State private var showScanner = false
    @State private var barcodeResult: BarcodeSearch?

    var body: some View {
        NavigationStack {
            Form {
                Picker("Mode", selection: $tab) {
                    Text("Search").tag(0)
                    Text("Scan").tag(1)
                    Text("Manual").tag(2)
                }
                .pickerStyle(.segmented)

                if tab == 0 {
                    searchSection
                } else if tab == 1 {
                    scanSection
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
            .onAppear { tab = initialTab }
            .sheet(isPresented: $showScanner) {
                NavigationStack {
                    BarcodeScannerView { code in
                        showScanner = false
                        barcode = digits(code)
                        Task { await lookupBarcode() }
                    }
                    .ignoresSafeArea()
                    .navigationTitle("Scan barcode")
                    .navigationBarTitleDisplayMode(.inline)
                    .toolbar {
                        ToolbarItem(placement: .cancellationAction) {
                            Button("Cancel") { showScanner = false }
                        }
                    }
                }
            }
        }
    }

    @ViewBuilder
    private var searchSection: some View {
        if !store.isPaired || !store.igdbConfigured {
            Section {
                Text("IGDB search needs a reachable server with IGDB keys. Use Scan or Manual, or pair in Settings.")
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
        resultSections
    }

    @ViewBuilder
    private var scanSection: some View {
        if !store.isPaired {
            Section {
                Text("Looking up a barcode needs a paired server. You can still scan or type the digits and add the game manually.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        Section {
            if BarcodeScannerView.isAvailable {
                Button("Scan box") { showScanner = true }
            } else {
                Text("Camera scanning needs a real iPhone. Type the digits below.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            TextField("UPC / EAN", text: $barcode)
                .keyboardType(.numberPad)
                .textInputAutocapitalization(.never)
            Button("Lookup") { Task { await lookupBarcode() } }
                .disabled(digits(barcode).count < 8 || searching || !store.isPaired)
        }
        if let owned = barcodeResult?.owned, !owned.isEmpty {
            Section("Already on the shelf") {
                ForEach(owned) { copy in
                    Text("\(copy.title) · \(copy.platform)")
                }
                Text("You can still add another copy.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        if searching {
            Section { ProgressView("Looking up…") }
        }
        if let title = barcodeResult?.productTitle, !title.isEmpty {
            Section("Catalog") {
                Text(title)
            }
        }
        resultSections
        if tab == 1, !searching, picked == nil, results.isEmpty, let product = barcodeResult?.productTitle, !product.isEmpty {
            Section("Add without IGDB") {
                TextField("Title", text: $title)
                TextField("Platform", text: $platform)
                regionCompleteness
                Button("Add to library") { addManual() }
                    .disabled(title.trimmingCharacters(in: .whitespaces).isEmpty || platform.trimmingCharacters(in: .whitespaces).isEmpty)
            }
        }
    }

    @ViewBuilder
    private var resultSections: some View {
        if let picked {
            Section {
                Text(picked.name).font(.headline)
                if picked.platforms.isEmpty {
                    TextField("Platform", text: $platform)
                } else {
                    Picker("Platform", selection: $pickPlatform) {
                        ForEach(picked.platforms) { p in
                            Text(p.name).tag(String(p.id))
                        }
                    }
                    .onAppear { selectPlatform(picked) }
                }
                regionCompleteness
                Button("Add to library") { Task { await addFromIGDB() } }
            }
            .onChange(of: picked.igdbId) { _, _ in
                selectPlatform(picked)
            }
        }
        if !results.isEmpty {
            Section(picked == nil ? "Matches" : "Other matches") {
                ForEach(results) { game in
                    Button {
                        picked = game
                        selectPlatform(game)
                    } label: {
                        HStack {
                            VStack(alignment: .leading) {
                                Text(game.name)
                                Text(game.platforms.map(\.name).joined(separator: ", "))
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                            Spacer()
                            if picked?.igdbId == game.igdbId {
                                Image(systemName: "checkmark")
                                    .foregroundStyle(.tint)
                            }
                        }
                    }
                    .buttonStyle(.plain)
                    .contentShape(Rectangle())
                }
            }
        }
    }

    private var manualSection: some View {
        Section {
            TextField("Title", text: $title)
            TextField("Platform", text: $platform)
            TextField("Barcode", text: $barcode)
                .keyboardType(.numberPad)
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

    private func digits(_ raw: String) -> String {
        raw.filter(\.isNumber)
    }

    private func preferredPlatform(_ game: SearchGame, hint: String? = nil) -> Int64 {
        let hint = (hint ?? barcodeResult?.platformHint)?.lowercased() ?? ""
        if !hint.isEmpty {
            if let match = game.platforms.first(where: { $0.name.lowercased() == hint }) {
                return match.id
            }
            if let match = game.platforms.first(where: { $0.name.lowercased().contains(hint) }) {
                return match.id
            }
        }
        return game.platforms.first?.id ?? 0
    }

    private func selectPlatform(_ game: SearchGame, hint: String? = nil) {
        let id = preferredPlatform(game, hint: hint)
        pickPlatform = id == 0 ? "" : String(id)
    }

    private func resolvedPlatformId(_ game: SearchGame) -> Int64? {
        if let id = Int64(pickPlatform), game.platforms.contains(where: { $0.id == id }) {
            return id
        }
        return game.platforms.first?.id
    }

    private func runSearch() async {
        error = nil
        searching = true
        defer { searching = false }
        do {
            results = try await store.api.search(q: search)
            picked = nil
            pickPlatform = ""
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func lookupBarcode() async {
        error = nil
        let code = digits(barcode)
        guard code.count >= 8 else { return }
        barcode = code
        searching = true
        defer { searching = false }
        do {
            let found = try await store.api.searchBarcode(code)
            barcodeResult = found
            barcode = found.barcode
            results = found.games
            if let first = found.games.first {
                picked = first
                selectPlatform(first, hint: found.platformHint)
            } else {
                picked = nil
                pickPlatform = ""
            }
            if let err = found.lookupError, !err.isEmpty, found.games.isEmpty {
                error = err
            }
            if found.games.isEmpty {
                if let q = found.query, !q.isEmpty {
                    title = q
                } else if let product = found.productTitle, !product.isEmpty {
                    title = product
                }
                if platform.isEmpty {
                    if let p = found.platform, !p.isEmpty {
                        platform = p
                    } else if let hint = found.platformHint, !hint.isEmpty {
                        platform = hint
                    }
                }
            }
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func addManual() {
        let code = digits(barcode)
        let item = LibraryItem.newLocal(
            title: title,
            platform: platform,
            region: region.isEmpty ? nil : region,
            completeness: completeness,
            notes: notes,
            barcode: code.isEmpty ? nil : code
        )
        try? store.add(item)
        dismiss()
    }

    private func addFromIGDB() async {
        guard let picked else { return }
        do {
            let code = digits(barcode)
            var item = try await store.api.createFromIGDB(
                gameId: picked.igdbId,
                platformId: resolvedPlatformId(picked),
                region: region.isEmpty ? nil : region,
                completeness: completeness,
                barcode: code.isEmpty ? nil : code
            )
            item.dirty = false
            try store.add(item)
            dismiss()
        } catch {
            self.error = error.localizedDescription
        }
    }
}
