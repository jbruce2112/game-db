import SwiftUI
import UIKit

struct AddGameView: View {
    @Environment(LibraryStore.self) private var store
    @Environment(\.dismiss) private var dismiss
    @State private var tab = 0
    @State private var title = ""
    @State private var platform = ""
    @State private var region = "us"
    @State private var completeness = ""
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
    @State private var addedCount = 0
    @State private var lastAdded: String?
    @State private var keepScanning = false
    @State private var showReview = false
    @State private var addedFromReview = false

    var body: some View {
        NavigationStack {
            Form {
                Picker("Mode", selection: $tab) {
                    Text("Scan").tag(0)
                    Text("Search").tag(1)
                    Text("Manual").tag(2)
                }
                .pickerStyle(.segmented)

                if let lastAdded {
                    Section {
                        Label("Added \(lastAdded)", systemImage: "checkmark.circle.fill")
                            .foregroundStyle(.green)
                        if addedCount > 1 {
                            Text("\(addedCount) added this session")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                }

                if tab == 0 {
                    scanSection
                    if searching {
                        Section { ProgressView("Looking up…") }
                    }
                } else if tab == 1 {
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
                    Button(addedCount > 0 ? "Done" : "Close") { dismiss() }
                }
            }
            .sheet(isPresented: $showReview, onDismiss: {
                if addedFromReview {
                    addedFromReview = false
                    return
                }
                keepScanning = false
                clearScanState()
            }) {
                ScanReviewSheet(
                    barcode: digits(barcode),
                    productTitle: barcodeResult?.productTitle ?? "",
                    owned: barcodeResult?.owned ?? [],
                    games: results,
                    platformHint: barcodeResult?.platformHint,
                    lookupError: barcodeResult?.lookupError,
                    coverBase: store.serverURL,
                    region: $region,
                    completeness: $completeness,
                    initialPicked: picked,
                    initialTitle: title,
                    initialPlatform: platform,
                    onCancel: {
                        keepScanning = false
                        showReview = false
                    },
                    onSaveIGDB: { game, platformId, notes in
                        try await addFromIGDB(game, platformId: platformId, notes: notes)
                    },
                    onSaveManual: { title, platform, notes in
                        persistManual(title: title, platform: platform, notes: notes)
                    },
                    onSaved: { name in
                        addedFromReview = true
                        showReview = false
                        didAdd(name)
                    }
                )
            }
            .sheet(isPresented: $showScanner) {
                NavigationStack {
                    BarcodeScannerView { code in
                        keepScanning = true
                        showScanner = false
                        barcode = digits(code)
                        Task { await lookupBarcode() }
                    }
                    .ignoresSafeArea()
                    .navigationTitle("Scan barcode")
                    .navigationBarTitleDisplayMode(.inline)
                    .toolbar {
                        ToolbarItem(placement: .cancellationAction) {
                            Button("Cancel") {
                                keepScanning = false
                                showScanner = false
                            }
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
            Text("Scan a box, then confirm the details before it is saved.")
                .font(.caption)
                .foregroundStyle(.secondary)
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
                Button("Add to library") { Task { await addFromIGDBPicked() } }
                    .disabled(!Self.completenessChosen(completeness))
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
            Button("Add to library") { addManual(title: title, platform: platform, notes: notes) }
                .disabled(title.trimmingCharacters(in: .whitespaces).isEmpty || platform.trimmingCharacters(in: .whitespaces).isEmpty || !Self.completenessChosen(completeness))
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
                Text("Select…").tag("")
                Text("Loose").tag("loose")
                Text("CIB").tag("cib")
                Text("New / sealed").tag("new")
            }
        }
    }

    static func completenessChosen(_ value: String) -> Bool {
        value == "loose" || value == "cib" || value == "new"
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
            showReview = true
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func addManual(title: String, platform: String, notes: String) {
        persistManual(title: title, platform: platform, notes: notes)
        didAdd(title)
    }

    private func persistManual(title: String, platform: String, notes: String) {
        guard Self.completenessChosen(completeness) else { return }
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
    }

    private func addFromIGDBPicked() async {
        guard let picked else { return }
        do {
            try await addFromIGDB(picked, platformId: resolvedPlatformId(picked), notes: notes)
            didAdd(picked.name)
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func addFromIGDB(_ game: SearchGame, platformId: Int64?, notes: String) async throws {
        let code = tab == 1 ? "" : digits(barcode)
        var item = try await store.api.createFromIGDB(
            gameId: game.igdbId,
            platformId: platformId,
            region: region.isEmpty ? nil : region,
            completeness: completeness,
            barcode: code.isEmpty ? nil : code
        )
        item.dirty = false
        try store.add(item)
    }

    private func clearScanState() {
        barcode = ""
        barcodeResult = nil
        results = []
        picked = nil
        pickPlatform = ""
        title = ""
        platform = ""
        notes = ""
    }

    private func didAdd(_ name: String) {
        addedCount += 1
        lastAdded = name
        error = nil
        search = ""
        notes = ""
        clearScanState()
        UINotificationFeedbackGenerator().notificationOccurred(.success)
        if tab == 0, keepScanning, BarcodeScannerView.isAvailable {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.55) {
                showScanner = true
            }
        }
    }
}

private struct ScanReviewSheet: View {
    let barcode: String
    let productTitle: String
    let owned: [OwnedCopy]
    let games: [SearchGame]
    let platformHint: String?
    let lookupError: String?
    let coverBase: String
    @Binding var region: String
    @Binding var completeness: String
    @State private var picked: SearchGame?
    @State private var pickPlatform: String = ""
    @State private var title: String
    @State private var platform: String
    @State private var notes: String = ""
    @State private var saving = false
    @State private var error: String?

    var onCancel: () -> Void
    var onSaveIGDB: (SearchGame, Int64?, String) async throws -> Void
    var onSaveManual: (String, String, String) -> Void
    var onSaved: (String) -> Void

    init(
        barcode: String,
        productTitle: String,
        owned: [OwnedCopy],
        games: [SearchGame],
        platformHint: String?,
        lookupError: String?,
        coverBase: String,
        region: Binding<String>,
        completeness: Binding<String>,
        initialPicked: SearchGame?,
        initialTitle: String,
        initialPlatform: String,
        onCancel: @escaping () -> Void,
        onSaveIGDB: @escaping (SearchGame, Int64?, String) async throws -> Void,
        onSaveManual: @escaping (String, String, String) -> Void,
        onSaved: @escaping (String) -> Void
    ) {
        self.barcode = barcode
        self.productTitle = productTitle
        self.owned = owned
        self.games = games
        self.platformHint = platformHint
        self.lookupError = lookupError
        self.coverBase = coverBase
        self._region = region
        self._completeness = completeness
        self.onCancel = onCancel
        self.onSaveIGDB = onSaveIGDB
        self.onSaveManual = onSaveManual
        self.onSaved = onSaved
        _picked = State(initialValue: initialPicked ?? games.first)
        _title = State(initialValue: initialTitle)
        _platform = State(initialValue: initialPlatform)
        if let game = initialPicked ?? games.first {
            let id = Self.preferredPlatform(game, hint: platformHint)
            _pickPlatform = State(initialValue: id == 0 ? "" : String(id))
        }
    }

    private var canSave: Bool {
        if !AddGameView.completenessChosen(completeness) { return false }
        if picked != nil { return true }
        return !title.trimmingCharacters(in: .whitespaces).isEmpty
            && !platform.trimmingCharacters(in: .whitespaces).isEmpty
    }

    var body: some View {
        NavigationStack {
            Form {
                if !owned.isEmpty {
                    Section("Already on the shelf") {
                        ForEach(owned) { copy in
                            Text("\(copy.title) · \(copy.platform)")
                        }
                        Text("You can still add another copy.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                if let err = lookupError, !err.isEmpty, games.isEmpty {
                    Section { Text(err).foregroundStyle(.secondary) }
                }
                if let picked {
                    Section {
                        HStack(alignment: .top, spacing: 12) {
                            cover(picked.coverUrl)
                            VStack(alignment: .leading, spacing: 4) {
                                Text(picked.name).font(.headline)
                                if !productTitle.isEmpty, productTitle != picked.name {
                                    Text(productTitle).font(.caption).foregroundStyle(.secondary)
                                }
                            }
                        }
                        if picked.platforms.isEmpty {
                            TextField("Platform", text: $platform)
                        } else {
                            Picker("Platform", selection: $pickPlatform) {
                                ForEach(picked.platforms) { p in
                                    Text(p.name).tag(String(p.id))
                                }
                            }
                        }
                    }
                    if games.count > 1 {
                        Section("Other matches") {
                            ForEach(games) { game in
                                Button {
                                    self.picked = game
                                    let id = Self.preferredPlatform(game, hint: platformHint)
                                    pickPlatform = id == 0 ? "" : String(id)
                                } label: {
                                    HStack {
                                        VStack(alignment: .leading) {
                                            Text(game.name)
                                            Text(game.platforms.map(\.name).joined(separator: ", "))
                                                .font(.caption)
                                                .foregroundStyle(.secondary)
                                        }
                                        Spacer()
                                        if picked.igdbId == game.igdbId {
                                            Image(systemName: "checkmark")
                                        }
                                    }
                                }
                                .buttonStyle(.plain)
                            }
                        }
                    }
                } else {
                    Section("Details") {
                        if !productTitle.isEmpty {
                            Text(productTitle).foregroundStyle(.secondary)
                        }
                        TextField("Title", text: $title)
                        TextField("Platform", text: $platform)
                    }
                }
                Section {
                    Picker("Region", selection: $region) {
                        Text("—").tag("")
                        Text("US").tag("us")
                        Text("EU").tag("eu")
                        Text("JP").tag("jp")
                        Text("AU").tag("au")
                        Text("Other").tag("other")
                    }
                    Picker("Completeness", selection: $completeness) {
                        Text("Select…").tag("")
                        Text("Loose").tag("loose")
                        Text("CIB").tag("cib")
                        Text("New / sealed").tag("new")
                    }
                    TextField("Notes", text: $notes, axis: .vertical)
                    if !barcode.isEmpty {
                        LabeledContent("Barcode", value: barcode)
                    }
                }
                if let error {
                    Section { Text(error).foregroundStyle(.red) }
                }
            }
            .navigationTitle("Review copy")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { onCancel() }
                }
            }
            .safeAreaInset(edge: .bottom) {
                HStack(spacing: 12) {
                    Button("Cancel") { onCancel() }
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 12)
                    Button {
                        Task { await save() }
                    } label: {
                        if saving {
                            ProgressView()
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, 12)
                        } else {
                            Text("Add")
                                .fontWeight(.semibold)
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, 12)
                        }
                    }
                    .disabled(saving || !canSave)
                    .background(canSave ? Color.accentColor : Color.gray.opacity(0.35), in: RoundedRectangle(cornerRadius: 12))
                    .foregroundStyle(canSave ? Color.black : Color.secondary)
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 10)
                .background(.bar)
            }
        }
    }

    @ViewBuilder
    private func cover(_ raw: String?) -> some View {
        let url = Self.coverURL(raw, base: coverBase)
        RoundedRectangle(cornerRadius: 6)
            .fill(Color(white: 0.12))
            .frame(width: 56, height: 74)
            .overlay {
                if let url {
                    AsyncImage(url: url) { phase in
                        if case .success(let image) = phase {
                            image.resizable().scaledToFit()
                        }
                    }
                }
            }
            .clipShape(RoundedRectangle(cornerRadius: 6))
    }

    private func save() async {
        saving = true
        error = nil
        defer { saving = false }
        do {
            if let picked {
                try await onSaveIGDB(picked, resolvedPlatformId(picked), notes)
                onSaved(picked.name)
            } else {
                onSaveManual(title, platform, notes)
                onSaved(title)
            }
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func resolvedPlatformId(_ game: SearchGame) -> Int64? {
        if let id = Int64(pickPlatform), game.platforms.contains(where: { $0.id == id }) {
            return id
        }
        return game.platforms.first?.id
    }

    static func preferredPlatform(_ game: SearchGame, hint: String?) -> Int64 {
        let hint = hint?.lowercased() ?? ""
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

    static func coverURL(_ raw: String?, base: String) -> URL? {
        guard let raw, !raw.isEmpty else { return nil }
        if let u = URL(string: raw), u.scheme != nil { return u }
        let root = base.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        guard !root.isEmpty else { return nil }
        return URL(string: root + (raw.hasPrefix("/") ? raw : "/" + raw))
    }
}
