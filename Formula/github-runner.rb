class GithubRunner < Formula
  desc "Self-hosted GitHub Actions runner"
  homepage "https://github.com/org/github-runner"
  license "MIT"

  on_macos do
    on_intel do
      url "https://github.com/org/github-runner/releases/download/v#{version}/github-runner_#{version}_darwin_amd64.zip"
    end
    on_arm do
      url "https://github.com/org/github-runner/releases/download/v#{version}/github-runner_#{version}_darwin_arm64.zip"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/org/github-runner/releases/download/v#{version}/github-runner_#{version}_linux_amd64.tar.gz"
    end
    on_arm do
      url "https://github.com/org/github-runner/releases/download/v#{version}/github-runner_#{version}_linux_arm64.tar.gz"
    end
  end

  def install
    bin.install "github-runner"

    # Install example config
    (etc/"github-runner").mkpath
    etc.install "configs/config.example.toml" => "github-runner/config.toml" unless (etc/"github-runner/config.toml").exist?

    # Create runtime directories
    (var/"github-runner").mkpath
    (var/"log").mkpath
    (var/"run").mkpath

    # Install shell completions
    bash_completion.install "scripts/completions/github-runner.bash" => "github-runner"
    zsh_completion.install "scripts/completions/github-runner.zsh" => "_github-runner"
    fish_completion.install "scripts/completions/github-runner.fish"
  end

  def caveats
    <<~EOS
      Configuration file: #{etc}/github-runner/config.toml
      Data directory:     #{var}/github-runner/
      Log directory:      #{var}/log/

      To start github-runner as a service:
        brew services start github-runner

      Before starting, edit the config file with your GitHub token:
        $EDITOR #{etc}/github-runner/config.toml
    EOS
  end

  service do
    run [opt_bin/"github-runner", "start", "--config", etc/"github-runner/config.toml"]
    keep_alive true
    log_path var/"log/github-runner.log"
    error_log_path var/"log/github-runner.log"
    working_dir var/"github-runner"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/github-runner version")
  end
end
