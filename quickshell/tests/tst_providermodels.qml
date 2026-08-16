import QtQuick
import QtTest
import "../components"

TestCase {
    id: testCase

    name: "ProviderModels"

    ListModel {
        id: providersModel
    }

    SettingsPanel {
        id: panel
    }

    function test_listModelStoresJsonString() {
        providersModel.append({
            "id": "deepseek",
            "label": "Deepseek",
            "baseUrl": "https://api.deepseek.com",
            "modelsJson": JSON.stringify(["a", "b"])
        });

        const entry = providersModel.get(0);
        const models = JSON.parse(entry.modelsJson);
        compare(models.length, 2);
        compare(String(models[0]), "a");
        compare(String(models[1]), "b");
    }

    function test_beginEditProviderCollectsModelIds() {
        panel.beginEditProvider("deepseek", "Deepseek", "https://api.deepseek.com", ["a", "b"]);
        compare(panel.providerFormModels.length, 2);
        compare(panel.providerFormModels[0], "a");
        compare(panel.providerFormModels[1], "b");
    }
}
