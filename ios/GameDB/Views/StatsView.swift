import SwiftUI

struct StatsView: View {
    @Environment(LibraryStore.self) private var store

    var body: some View {
        let stats = store.stats
        Group {
            if stats.total == 0 {
                ContentUnavailableView(
                    "Nothing on the shelf yet",
                    systemImage: "chart.bar",
                    description: Text("Add a game to see counts by platform, region, and completeness.")
                )
            } else {
                List {
                    Section {
                        hero("Games", value: "\(stats.total)")
                        hero("Platforms", value: "\(stats.platforms.count)")
                        hero("With cover", value: ShelfStats.percent(stats.withCover, of: stats.total))
                        hero("With barcode", value: ShelfStats.percent(stats.withBarcode, of: stats.total))
                    }
                    bars("Platforms", rows: stats.platforms, total: stats.total)
                    bars("Region", rows: stats.regions, total: stats.total)
                    bars("Completeness", rows: stats.completeness, total: stats.total)
                }
            }
        }
        .navigationTitle("Statistics")
        .preferredColorScheme(.dark)
    }

    private func hero(_ label: String, value: String) -> some View {
        HStack {
            Text(label)
            Spacer()
            Text(value)
                .font(.body.monospacedDigit().weight(.semibold))
                .foregroundStyle(Color(red: 0.89, green: 0.69, blue: 0.29))
        }
    }

    private func bars(_ title: String, rows: [ShelfRow], total: Int) -> some View {
        Section(title) {
            ForEach(rows, id: \.name) { row in
                VStack(alignment: .leading, spacing: 6) {
                    HStack {
                        Text(row.name)
                            .lineLimit(1)
                        Spacer()
                        Text("\(row.count)")
                            .font(.subheadline.monospacedDigit())
                            .foregroundStyle(.secondary)
                        Text(ShelfStats.percent(row.count, of: total))
                            .font(.caption.monospacedDigit())
                            .foregroundStyle(.secondary)
                            .frame(width: 40, alignment: .trailing)
                    }
                    GeometryReader { geo in
                        Capsule()
                            .fill(Color(white: 0.18))
                            .overlay(alignment: .leading) {
                                Capsule()
                                    .fill(Color(red: 0.89, green: 0.69, blue: 0.29))
                                    .frame(width: total > 0 ? geo.size.width * CGFloat(row.count) / CGFloat(total) : 0)
                            }
                    }
                    .frame(height: 6)
                }
                .padding(.vertical, 2)
            }
        }
    }
}
