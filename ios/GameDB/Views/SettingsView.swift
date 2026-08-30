import SwiftUI
import UniformTypeIdentifiers

struct SettingsView: View {
    @Environment(LibraryStore.self) private var store
    @Environment(\.dismiss) private var dismiss
    @State private var url: String = ""
    @State private var password: String = ""
    @State private var showImporter = false
    @State private var confirmImport = false
    @State private var pendingCSV: Data?

    var body: some View {
        NavigationStack {
            Form {
                Section("Server") {
                    TextField("http://192.168.1.10:8080", text: $url)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                    SecureField("Password", text: $password)
                    Button("Connect") {
                        Task { await store.pair(urlString: url.trimmingCharacters(in: .whitespaces), password: password) }
                    }
                    .disabled(url.isEmpty || password.isEmpty)
                    if store.isPaired {
                        Button("Forget server", role: .destructive) {
                            store.forgetServer()
                            password = ""
                        }
                    }
                }
                Section("Library") {
                    Button("Import CSV") { showImporter = true }
                    Text("Replaces the entire library. If a server is connected, it is wiped too.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Section("Status") {
                    LabeledContent("Paired", value: store.isPaired ? "Yes" : "No")
                    LabeledContent("Reachable", value: store.online ? "Yes" : "No")
                    LabeledContent("IGDB", value: store.igdbConfigured ? "Configured" : "No")
                    LabeledContent("Prices", value: store.pricechartingConfigured ? "Configured" : "No")
                    if let cents = store.shelfValueCents {
                        LabeledContent("Shelf value", value: PriceQuote.usd(cents))
                    }
                    if let last = store.lastSync {
                        LabeledContent("Last sync", value: last.formatted())
                    }
                }
                if let err = store.errorMessage {
                    Section { Text(err).foregroundStyle(.red) }
                }
            }
            .navigationTitle("Settings")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
            .onAppear { url = store.serverURL }
            .fileImporter(
                isPresented: $showImporter,
                allowedContentTypes: [.commaSeparatedText, .plainText]
            ) { result in
                switch result {
                case .success(let file):
                    guard file.startAccessingSecurityScopedResource() else {
                        store.errorMessage = "Could not read file"
                        return
                    }
                    defer { file.stopAccessingSecurityScopedResource() }
                    do {
                        pendingCSV = try Data(contentsOf: file)
                        confirmImport = true
                    } catch {
                        store.errorMessage = error.localizedDescription
                    }
                case .failure(let err):
                    store.errorMessage = err.localizedDescription
                }
            }
            .confirmationDialog(
                "Replace entire library with this CSV?",
                isPresented: $confirmImport,
                titleVisibility: .visible
            ) {
                Button("Replace library", role: .destructive) {
                    if let pendingCSV {
                        Task { await store.importCSV(pendingCSV) }
                    }
                    pendingCSV = nil
                }
                Button("Cancel", role: .cancel) { pendingCSV = nil }
            }
        }
    }
}
