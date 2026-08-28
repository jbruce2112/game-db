import SwiftUI

struct SettingsView: View {
    @Environment(LibraryStore.self) private var store
    @Environment(\.dismiss) private var dismiss
    @State private var url: String = ""
    @State private var password: String = ""

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
                Section("Status") {
                    LabeledContent("Paired", value: store.isPaired ? "Yes" : "No")
                    LabeledContent("Reachable", value: store.online ? "Yes" : "No")
                    LabeledContent("IGDB", value: store.igdbConfigured ? "Configured" : "No")
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
        }
    }
}
