import SwiftUI

struct PriceCheckView: View {
    @Environment(LibraryStore.self) private var store
    @Environment(\.dismiss) private var dismiss
    @State private var tab = 0
    @State private var search = ""
    @State private var barcode = ""
    @State private var results: [SearchGame] = []
    @State private var picked: SearchGame?
    @State private var pickPlatform = ""
    @State private var productTitle = ""
    @State private var productPlatform = ""
    @State private var quote: PriceCheckResult?
    @State private var searching = false
    @State private var pricing = false
    @State private var error: String?
    @State private var showScanner = false

    var body: some View {
        NavigationStack {
            Form {
                Picker("Mode", selection: $tab) {
                    Text("Scan").tag(0)
                    Text("Search").tag(1)
                }
                .pickerStyle(.segmented)
                .onChange(of: tab) { _, _ in
                    clearResults()
                }

                if !store.isPaired {
                    Section {
                        Text("Price check needs a paired server with eBay or PriceCharting configured.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                } else if !store.pricechartingConfigured {
                    Section {
                        Text("Set eBay or PriceCharting keys on the server to look up values.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }

                if tab == 0 {
                    scanSection
                } else {
                    searchSection
                }

                if searching {
                    Section { ProgressView("Looking up…") }
                }
                if pricing {
                    Section { ProgressView("Looking up prices…") }
                }
                if let error {
                    Section { Text(error).foregroundStyle(.red) }
                }
                if picked != nil || !productTitle.isEmpty || quote != nil {
                    resultSection
                }
            }
            .navigationTitle("Price check")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                }
            }
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
    private var scanSection: some View {
        Section {
            Text("Scan or type a barcode. Prices are shown without adding the game.")
                .font(.caption)
                .foregroundStyle(.secondary)
            if BarcodeScannerView.isAvailable {
                Button("Scan box") { showScanner = true }
                    .disabled(!store.isPaired)
            }
            TextField("UPC / EAN", text: $barcode)
                .keyboardType(.numberPad)
                .textInputAutocapitalization(.never)
            Button("Lookup") { Task { await lookupBarcode() } }
                .disabled(digits(barcode).count < 8 || searching || pricing || !store.isPaired)
        }
    }

    @ViewBuilder
    private var searchSection: some View {
        if !store.isPaired || !store.igdbConfigured {
            Section {
                Text("IGDB search needs a reachable server with IGDB keys. You can still scan a barcode.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        Section {
            HStack {
                TextField("Title", text: $search)
                Button("Go") { Task { await runSearch() } }
                    .disabled(search.trimmingCharacters(in: .whitespaces).isEmpty || searching || pricing || !store.isPaired)
            }
        }
        if !results.isEmpty {
            Section("Matches") {
                ForEach(results) { game in
                    Button {
                        picked = game
                        selectPlatform(game)
                        Task { await fetchPrices() }
                    } label: {
                        HStack {
                            remoteCover(game.coverUrl)
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

    @ViewBuilder
    private var resultSection: some View {
        Section {
            HStack(alignment: .top, spacing: 12) {
                remoteCover(picked?.coverUrl, large: true)
                VStack(alignment: .leading, spacing: 4) {
                    Text(displayTitle)
                        .font(.headline)
                    if !displayPlatform.isEmpty {
                        Text(displayPlatform)
                            .font(.subheadline)
                            .foregroundStyle(.secondary)
                    }
                    if !digits(barcode).isEmpty, tab == 0 {
                        Text(digits(barcode))
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            }
            if let picked, picked.platforms.count > 1 {
                Picker("Platform", selection: $pickPlatform) {
                    ForEach(picked.platforms) { p in
                        Text(p.name).tag(String(p.id))
                    }
                }
                .onChange(of: pickPlatform) { _, _ in
                    Task { await fetchPrices() }
                }
            }
        }
        if let quote {
            Section(quote.value?.source == "ebay" ? "eBay asking" : "Prices") {
                if quote.status != "ok" || quote.value == nil {
                    Text("No listings found for this title.")
                        .foregroundStyle(.secondary)
                } else if let value = quote.value {
                    if !value.productName.isEmpty {
                        LabeledContent("Match", value: value.productName)
                    }
                    if let n = value.listings, n > 0 {
                        LabeledContent("Listings", value: String(n))
                    }
                    LabeledContent("Loose", value: value.looseCents.map(PriceQuote.usd) ?? "—")
                    LabeledContent("CIB", value: value.cibCents.map(PriceQuote.usd) ?? "—")
                    LabeledContent("New", value: value.newCents.map(PriceQuote.usd) ?? "—")
                    if let url = URL(string: value.url), !value.url.isEmpty {
                        Link(value.source == "ebay" ? "See listings on eBay" : "Open on PriceCharting", destination: url)
                    }
                }
            }
        }
    }

    private var displayTitle: String {
        picked?.name ?? (productTitle.isEmpty ? search : productTitle)
    }

    private var displayPlatform: String {
        if let picked {
            return resolvedPlatformName(picked)
        }
        return productPlatform
    }

    @ViewBuilder
    private func remoteCover(_ urlString: String?, large: Bool = false) -> some View {
        let w: CGFloat = large ? 72 : 40
        let h: CGFloat = large ? 96 : 54
        Group {
            if let urlString, let url = URL(string: urlString) {
                AsyncImage(url: url) { phase in
                    switch phase {
                    case .success(let image):
                        image.resizable().aspectRatio(contentMode: .fill)
                    default:
                        Color(white: 0.12)
                    }
                }
            } else {
                Color(white: 0.12)
            }
        }
        .frame(width: w, height: h)
        .clipShape(RoundedRectangle(cornerRadius: 6))
    }

    private func digits(_ raw: String) -> String {
        raw.filter(\.isNumber)
    }

    private func selectPlatform(_ game: SearchGame, hint: String? = nil) {
        let hint = hint?.lowercased() ?? ""
        if !hint.isEmpty {
            if let match = game.platforms.first(where: { $0.name.lowercased() == hint }) {
                pickPlatform = String(match.id)
                return
            }
            if let match = game.platforms.first(where: { $0.name.lowercased().contains(hint) }) {
                pickPlatform = String(match.id)
                return
            }
        }
        pickPlatform = game.platforms.first.map { String($0.id) } ?? ""
    }

    private func resolvedPlatformName(_ game: SearchGame) -> String {
        if let id = Int64(pickPlatform), let match = game.platforms.first(where: { $0.id == id }) {
            return match.name
        }
        return game.platforms.first?.name ?? productPlatform
    }

    private func clearResults() {
        results = []
        picked = nil
        pickPlatform = ""
        productTitle = ""
        productPlatform = ""
        quote = nil
        error = nil
    }

    private func runSearch() async {
        error = nil
        quote = nil
        searching = true
        var shouldPrice = false
        defer { searching = false }
        do {
            results = try await store.api.search(q: search)
            picked = nil
            pickPlatform = ""
            productTitle = ""
            if results.isEmpty {
                productTitle = search.trimmingCharacters(in: .whitespaces)
                shouldPrice = true
            }
        } catch {
            self.error = error.localizedDescription
        }
        if shouldPrice {
            searching = false
            await fetchPrices()
        }
    }

    private func lookupBarcode() async {
        error = nil
        quote = nil
        let code = digits(barcode)
        guard code.count >= 8 else { return }
        barcode = code
        searching = true
        var shouldPrice = false
        defer { searching = false }
        do {
            let found = try await store.api.searchBarcode(code)
            barcode = found.barcode
            results = found.games
            productTitle = found.query ?? found.productTitle ?? ""
            productPlatform = found.platform ?? found.platformHint ?? ""
            if let first = found.games.first {
                picked = first
                selectPlatform(first, hint: found.platformHint)
            } else {
                picked = nil
                pickPlatform = ""
            }
            if let err = found.lookupError, !err.isEmpty, found.games.isEmpty, productTitle.isEmpty {
                error = err
            }
            shouldPrice = true
        } catch {
            self.error = error.localizedDescription
        }
        if shouldPrice {
            searching = false
            await fetchPrices()
        }
    }

    private func fetchPrices() async {
        guard store.isPaired else { return }
        let title: String
        let platform: String
        if let picked {
            title = picked.name
            platform = resolvedPlatformName(picked)
        } else {
            title = productTitle
            platform = productPlatform
        }
        let code = tab == 0 ? digits(barcode) : ""
        guard !title.isEmpty || code.count >= 8 else { return }
        pricing = true
        defer { pricing = false }
        do {
            quote = try await store.api.checkPrice(
                title: title,
                platform: platform,
                barcode: code.isEmpty ? nil : code
            )
        } catch {
            self.error = error.localizedDescription
        }
    }
}
