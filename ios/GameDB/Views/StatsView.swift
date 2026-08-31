import Charts
import SwiftUI

struct StatsView: View {
    @Environment(LibraryStore.self) private var store
    @State private var yearPlatform = ""

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
                    yearChart(stats)
                    if let shelf = stats.shelfCents, stats.priced > 0 {
                        Section {
                            hero("Shelf", value: PriceQuote.usd(shelf))
                            if let median = stats.medianCents {
                                hero("Median", value: PriceQuote.usd(median))
                            }
                            hero("Priced", value: ShelfStats.percent(stats.priced, of: stats.total))
                            if let top = stats.mostExpensive.first {
                                hero("Highest", value: PriceQuote.usd(top.cents))
                            }
                        } header: {
                            Text("Asking prices")
                        } footer: {
                            Text("eBay asking prices. Completeness “unknown” uses the CIB quote when present.")
                        }
                        moneyBars("Value by platform", rows: stats.valueByPlatform, total: shelf)
                        expensive("Most expensive", rows: stats.mostExpensive)
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

    private func yearChart(_ stats: ShelfStats) -> some View {
        let rows = stats.countsByYear(platform: yearPlatform)
        return Section {
            Picker("Platform", selection: $yearPlatform) {
                Text("All platforms").tag("")
                ForEach(stats.platforms, id: \.name) { row in
                    Text(row.name).tag(row.name)
                }
            }
            if rows.isEmpty {
                Text("No added dates to chart.")
                    .foregroundStyle(.secondary)
            } else {
                Chart(rows, id: \.year) { row in
                    LineMark(
                        x: .value("Year", row.year),
                        y: .value("Games", row.count)
                    )
                    .foregroundStyle(Color(red: 0.89, green: 0.69, blue: 0.29))
                    .interpolationMethod(.linear)
                    PointMark(
                        x: .value("Year", row.year),
                        y: .value("Games", row.count)
                    )
                    .foregroundStyle(Color(red: 0.89, green: 0.69, blue: 0.29))
                }
                .chartXScale(domain: (rows.first?.year ?? 0)...(rows.last?.year ?? 0))
                .chartXAxis {
                    AxisMarks(values: rows.map(\.year)) { value in
                        AxisGridLine()
                        AxisTick()
                        AxisValueLabel {
                            if let year = value.as(Int.self) {
                                Text(String(year)).font(.caption2)
                            }
                        }
                    }
                }
                .chartYAxis {
                    AxisMarks(position: .leading)
                }
                .frame(height: 180)
                .padding(.vertical, 4)
                .accessibilityLabel(yearChartLabel(rows))
            }
        } header: {
            Text("Added by year")
        } footer: {
            Text("Uses each copy’s added date. Empty years are shown as zero.")
        }
    }

    private func yearChartLabel(_ rows: [ShelfYearRow]) -> String {
        rows.map { "\($0.year): \($0.count)" }.joined(separator: ", ")
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
                    bar(width: total > 0 ? CGFloat(row.count) / CGFloat(total) : 0)
                }
                .padding(.vertical, 2)
            }
        }
    }

    private func moneyBars(_ title: String, rows: [ShelfValueRow], total: Int) -> some View {
        Section(title) {
            ForEach(rows, id: \.name) { row in
                VStack(alignment: .leading, spacing: 6) {
                    HStack {
                        Text(row.name)
                            .lineLimit(1)
                        Spacer()
                        Text(PriceQuote.usd(row.cents))
                            .font(.subheadline.monospacedDigit())
                            .foregroundStyle(.secondary)
                        Text(ShelfStats.percent(row.cents, of: total))
                            .font(.caption.monospacedDigit())
                            .foregroundStyle(.secondary)
                            .frame(width: 40, alignment: .trailing)
                    }
                    bar(width: total > 0 ? CGFloat(row.cents) / CGFloat(total) : 0)
                }
                .padding(.vertical, 2)
            }
        }
    }

    private func expensive(_ title: String, rows: [ShelfPriceRow]) -> some View {
        Section(title) {
            ForEach(rows, id: \.id) { row in
                NavigationLink {
                    GameDetailView(itemID: row.id)
                } label: {
                    HStack(alignment: .firstTextBaseline) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(row.title)
                                .lineLimit(1)
                            Text(row.platform)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                                .lineLimit(1)
                        }
                        Spacer()
                        Text(PriceQuote.usd(row.cents))
                            .font(.body.monospacedDigit())
                            .foregroundStyle(Color(red: 0.89, green: 0.69, blue: 0.29))
                    }
                }
            }
        }
    }

    private func bar(width: CGFloat) -> some View {
        GeometryReader { geo in
            Capsule()
                .fill(Color(white: 0.18))
                .overlay(alignment: .leading) {
                    Capsule()
                        .fill(Color(red: 0.89, green: 0.69, blue: 0.29))
                        .frame(width: geo.size.width * min(max(width, 0), 1))
                }
        }
        .frame(height: 6)
    }
}
