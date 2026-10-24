# Contributing

Follow the [README's build and test instructions](README.md#building-and-testing)
for native SDK setup, formatting, linting, and race tests. Keep Go guides in `docs/`
and API explanations in exported Go declarations' comments. Changes to the Rust
core and the C ABI belong in `qortoo-rs`.

## Developer website checks

The **Developer website** PR check runs for documentation and API inputs and
builds this repository's guides and Go API reference from the PR's exact head commit, including fork heads. It calls the
shared [website workflow](https://github.com/qortoo/qortoo/blob/main/.github/workflows/build-docs.yml)
with read-only permissions and no inherited secrets. Candidate builds do not deploy.

PR validation and main refresh use the same [path filters](.github/workflows/developer-site.yml).
Metadata-only and unrelated workflow changes skip these checks; mixed changes run.
Manual main refresh remains available.

For a local portal preview, place `qortoo` and `qortoo-rs` checkouts beside this
repository and follow the website's
[development and candidate-validation guide](https://github.com/qortoo/qortoo/blob/main/docs/developer-site.md).
Use `QORTOO_GO_REF=HEAD` to include your committed Go change. API extraction reads
declarations without compiling cgo or requiring native headers and libraries.

A `main` push that changes documentation or API inputs requests a website refresh
through a separate job. Maintainers configure `WORKFLOW_PAT` with
Actions write access to `qortoo/qortoo`; PR validation never receives it. To retry,
manually run **Developer website** on `main`, or use **Documentation** in the
website's Actions tab with candidate SDK set to `none`. A failed build leaves the
published website in place.
