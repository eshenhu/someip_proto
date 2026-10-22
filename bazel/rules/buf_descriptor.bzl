"""Rules for producing complete, source-aware FileDescriptorSets with Buf."""

_BUF_TOOLCHAIN_TYPE = "@rules_buf//tools/buf:toolchain_type"


def _buf_descriptor_impl(ctx):
    output = ctx.actions.declare_file(ctx.label.name + ".fds.binpb")
    cache_dir = ctx.actions.declare_directory(ctx.label.name + ".buf-cache")
    arguments = ctx.actions.args()
    arguments.add("build")
    arguments.add(ctx.attr.module)
    arguments.add("--as-file-descriptor-set")
    arguments.add("--output")
    arguments.add(output.path)

    ctx.actions.run(
        executable = ctx.toolchains[_BUF_TOOLCHAIN_TYPE].cli,
        arguments = [arguments],
        inputs = depset(ctx.files.srcs),
        outputs = [output, cache_dir],
        env = {
            "BUF_CACHE_DIR": cache_dir.path,
        },
        mnemonic = "BufDescriptorSet",
        progress_message = "Building source-aware descriptor set %{label}",
    )

    return [DefaultInfo(files = depset([output]))]


buf_descriptor = rule(
    implementation = _buf_descriptor_impl,
    attrs = {
        "module": attr.string(
            mandatory = True,
            doc = "Workspace-relative path to the Buf module root.",
        ),
        "srcs": attr.label_list(
            mandatory = True,
            allow_files = [".lock", ".proto", ".yaml"],
            doc = "Complete local Buf module closure, including buf.yaml and buf.lock.",
        ),
    },
    toolchains = [_BUF_TOOLCHAIN_TYPE],
    doc = "Builds a complete FileDescriptorSet with imports and SourceCodeInfo.",
)


def _descriptor_smoke_test_impl(ctx):
    executable = ctx.actions.declare_file(ctx.label.name)
    expected_string_checks = "\n".join([
        """if ! grep -aq {expected_string} \"${{descriptor_set}}\"; then
  echo \"descriptor set does not contain expected string: {expected_string}\" >&2
  exit 1
fi""".format(expected_string = repr(expected_string))
        for expected_string in ctx.attr.expected_strings
    ])
    ctx.actions.write(
        output = executable,
        is_executable = True,
        content = """#!/usr/bin/env bash
set -euo pipefail

descriptor_set="${{RUNFILES_DIR}}/${{TEST_WORKSPACE}}/{descriptor_path}"

if [[ ! -s "${{descriptor_set}}" ]]; then
  echo "descriptor set was not produced" >&2
  exit 1
fi

{expected_string_checks}
""".format(
        descriptor_path = ctx.file.descriptor.short_path,
        expected_string_checks = expected_string_checks,
    ),
    )

    return [DefaultInfo(
        executable = executable,
        runfiles = ctx.runfiles(files = [ctx.file.descriptor]),
    )]


descriptor_smoke_test = rule(
    implementation = _descriptor_smoke_test_impl,
    attrs = {
        "descriptor": attr.label(
            mandatory = True,
            allow_single_file = True,
        ),
        "expected_strings": attr.string_list(
            doc = "Additional UTF-8 strings expected in the descriptor set.",
        ),
    },
    test = True,
    doc = "Checks that an FDS contains imported descriptors and SourceCodeInfo.",
)
