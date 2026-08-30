import SwiftUI

struct GameDetailView: View {
    @Environment(LibraryStore.self) private var store
    @Environment(\.dismiss) private var dismiss
    var itemID: String

    @State private var title = ""
    @State private var platform = ""
    @State private var region = ""
    @State private var completeness = "unknown"
    @State private var notes = ""
    @State private var barcode = ""

    private var item: LibraryItem? {
        store.items.first(where: { $0.id == itemID && ($0.deletedAt ?? "").isEmpty })
    }

    var body: some View {
        Form {
            if let item {
                Section {
                    CoverView(item: item)
                        .aspectRatio(3/4, contentMode: .fit)
                        .frame(maxWidth: 180)
                        .frame(maxWidth: .infinity)
                        .listRowBackground(Color.clear)
                }
                Section("Details") {
                    TextField("Title", text: $title)
                    TextField("Platform", text: $platform)
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
                    TextField("Barcode", text: $barcode)
                        .keyboardType(.numberPad)
                    TextField("Notes", text: $notes, axis: .vertical)
                        .lineLimit(3...6)
                }
                if let quote = store.quotes[item.id] {
                    Section("PriceCharting") {
                        LabeledContent("Match", value: quote.productName)
                        if !quote.consoleName.isEmpty {
                            LabeledContent("Console", value: quote.consoleName)
                        }
                        if let cents = quote.looseCents {
                            LabeledContent("Loose", value: PriceQuote.usd(cents))
                        }
                        if let cents = quote.cibCents {
                            LabeledContent("CIB", value: PriceQuote.usd(cents))
                        }
                        if let cents = quote.newCents {
                            LabeledContent("New", value: PriceQuote.usd(cents))
                        }
                        if let url = URL(string: quote.url), !quote.url.isEmpty {
                            Link("Open on PriceCharting", destination: url)
                        }
                    }
                }
                Section {
                    Button("Save") { save(item) }
                    Button("Delete", role: .destructive) {
                        try? store.delete(item)
                        dismiss()
                    }
                }
            } else {
                Text("Not found")
            }
        }
        .navigationTitle("Game")
        .navigationBarTitleDisplayMode(.inline)
        .onAppear { load() }
    }

    private func load() {
        guard let item else { return }
        title = item.title
        platform = item.platform
        region = item.region ?? ""
        completeness = item.completeness
        notes = item.notes
        barcode = item.barcode ?? ""
    }

    private func save(_ item: LibraryItem) {
        var next = item.touching()
        next.title = title
        next.platform = platform
        next.region = region.isEmpty ? nil : region
        next.completeness = completeness
        next.notes = notes
        let code = barcode.filter(\.isNumber)
        next.barcode = code.isEmpty ? nil : code
        try? store.upsert(next)
        dismiss()
    }
}
