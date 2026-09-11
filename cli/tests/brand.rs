use std::process::Command;

#[test]
fn new_command_exposes_brand_and_version() {
    let binary = option_env!("CARGO_BIN_EXE_pluginpocket");
    assert!(
        binary.is_some(),
        "pluginpocket executable must be installed"
    );
    let binary = binary.unwrap();
    let output = Command::new(binary).arg("--help").output().unwrap();
    assert!(output.status.success());
    let help = String::from_utf8(output.stdout).unwrap();
    assert!(help.contains("PluginPocket"), "{help}");
    assert!(help.contains("pluginpocket"), "{help}");
    let output = Command::new(binary).arg("version").output().unwrap();
    assert!(output.status.success());
    assert_eq!(
        String::from_utf8(output.stdout).unwrap().trim(),
        format!("pluginpocket {}", env!("CARGO_PKG_VERSION"))
    );
}
