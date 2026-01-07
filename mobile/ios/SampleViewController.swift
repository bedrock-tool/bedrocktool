import UIKit
// After binding, import the generated framework module name, e.g. `BedrockTool`
// import BedrockTool

class SampleViewController: UIViewController {
    override func viewDidLoad() {
        super.viewDidLoad()
        // Example call
        requestPacks(ip: "1.2.3.4")
    }

    func requestPacks(ip: String) {
        DispatchQueue.global(qos: .userInitiated).async {
            do {
                // Replace with actual generated call name after you bind. For example:
                // let res = try BedrockToolRequestPacks(ip)
                // The binding returns either a URL string or a local file path to the zip.

                // Example pseudocode:
                let res = try self.callMobileRequestPacks(ip: ip)
                DispatchQueue.main.async {
                    self.handleResult(res)
                }
            } catch {
                DispatchQueue.main.async {
                    self.showError(error.localizedDescription)
                }
            }
        }
    }

    // Pseudocode wrapper - replace with actual binding call
    func callMobileRequestPacks(ip: String) throws -> String {
        // Example - the real signature will depend on the generated API
        // return try BedrockToolRequestPacks(ip)
        return ""
    }

    func handleResult(_ s: String) {
        if s.hasPrefix("file:") || FileManager.default.fileExists(atPath: s) {
            // It's a local file path: present/share/unzip
            let alert = UIAlertController(title: "Got file", message: s, preferredStyle: .alert)
            alert.addAction(UIAlertAction(title: "OK", style: .default))
            present(alert, animated: true)
        } else if let url = URL(string: s) {
            // It's a URL: you can download it
            let alert = UIAlertController(title: "Got URL", message: url.absoluteString, preferredStyle: .alert)
            alert.addAction(UIAlertAction(title: "OK", style: .default))
            present(alert, animated: true)
        } else {
            showError("Unknown response: \(s)")
        }
    }

    func showError(_ msg: String) {
        let alert = UIAlertController(title: "Error", message: msg, preferredStyle: .alert)
        alert.addAction(UIAlertAction(title: "OK", style: .default))
        present(alert, animated: true)
    }
}
