#!/usr/bin/env python3

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

from pathlib import Path

import argparse
import json
import os
import shlex
import subprocess
import sys


def get_nsbox_release_info():
    release_files = Path(__file__).parent.parent / 'release'

    try:
        with (release_files / 'VERSION').open() as fp:
            nsbox_version = fp.read().strip()
    except FileNotFoundError:
        nsbox_version = None

    try:
        with (release_files / 'BRANCH').open() as fp:
            nsbox_branch = fp.read().strip()
    except FileNotFoundError:
        nsbox_branch = None

    return nsbox_version, nsbox_branch


def read_metadata(image, extra_vars):
    with (image / 'metadata.json').open() as fp:
        metadata = json.load(fp)

    if 'base' not in metadata or 'valid_tags' not in metadata:
        sys.exit('Metadata must specify base and valid_tags.')

    if 'remote' not in metadata and 'target' not in metadata:
        sys.exit('Metadata must specify at least one of remote and target.')

    string_keys = {'base', 'remote', 'target', 'parent', 'sudo_group'}
    if not all(isinstance(metadata.get(key, ''), str) for key in string_keys):
        sys.exit('Metadata base, remote, and target must be strings.')

    if (not isinstance(metadata['valid_tags'], list)
            or not all(isinstance(tag, str)
                       for tag in metadata['valid_tags'])):
        sys.exit('Metadata valid_tags must be a list of strings.')

    for key in string_keys:
        if key in metadata:
            metadata[key] = metadata[key].format(**extra_vars)

    if 'target' not in metadata:
        metadata['target'] = metadata['remote']

    return metadata


def run(*args, **kw):
    kw['check'] = True

    try:
        subprocess.run(*args, **kw)
    except subprocess.CalledProcessError as ex:
        sys.exit(ex.returncode)


def export_image(metadata, target, builder):
    BUILDERS_TO_EXPORTERS = {
        'docker': 'docker',
        # XXX: We need podman in order to export images built with buildah.
        'buildah': 'podman',
    }

    print('Exporting image...')
    run([
        BUILDERS_TO_EXPORTERS[builder], 'save', '-o', target,
        metadata['target']
    ])


def main():
    parser = argparse.ArgumentParser(prog='nsbox-bender',
                                     description='Build an image')

    parser.add_argument('image',
                        help='The path to the image directory to build',
                        type=Path)
    parser.add_argument('--debug', action='store_true')
    parser.add_argument(
        '-x',
        '--export',
        help='Export the image to the given archive after building')
    parser.add_argument('--builder',
                        help='The builder to use',
                        choices=['docker', 'buildah'],
                        default='buildah')
    parser.add_argument('--force-color',
                        help='Force colored output',
                        action='store_true')
    parser.add_argument('--extra-bender-args',
                        help='Extra args to pass to ansible-bender')
    parser.add_argument('--extra-ansible-args',
                        help='Extra args to pass to ansible-playbook',
                        default='')

    parser.add_argument('--override-nsbox-version',
                        help='Override the nsbox release version')
    parser.add_argument('--override-nsbox-branch',
                        help='Override the nsbox release branch',
                        choices=['edge', 'stable'])

    args = parser.parse_args()

    nsbox_version, nsbox_branch = get_nsbox_release_info()

    if args.override_nsbox_version is not None:
        nsbox_version = args.override_nsbox_version

    if args.override_nsbox_branch is not None:
        nsbox_branch = args.override_nsbox_branch

    if nsbox_version is None or nsbox_branch is None:
        sys.exit(
            'Could not find version and branch, and --override arguments were not given.'
        )

    assert nsbox_branch in ('edge', 'stable'), nsbox_branch

    if nsbox_branch == 'edge':
        product_name = 'nsbox-edge'
    else:
        product_name = 'nsbox'

    image = args.image
    image_tag = None
    if ':' in image.name:
        image_basename, image_tag = image.name.rsplit(':', 1)
        image = image.parent / image_basename

    # XXX: Similar code to internal/image/image.go.

    extra_vars = {
        'image_tag': image_tag or '',
        'nsbox_branch': nsbox_branch,
        'nsbox_product_name': product_name,
        'nsbox_version': nsbox_version,
    }

    metadata = read_metadata(image, extra_vars)

    if metadata['valid_tags']:
        if image_tag is None:
            sys.exit('Metadata requires a tag but none was given.')
        elif image_tag not in metadata['valid_tags']:
            sys.exit(
                f'Invalid tag (valid choices are {", ".join(metadata["valid_tags"])})'
            )
    elif image_tag is not None:
        sys.exit('Metadata does not allow any tags but one was given.')

    base = metadata['base']

    if base == '@local':
        base = metadata['target'] + '-bud'

        run([
            args.builder, 'bud' if args.builder == 'buildah' else 'build',
            *(f'--build-arg={k.upper()}={v}' for k, v in extra_vars.items()),
            f'--tag={base}',
            str(image)
        ])

    command = ['ansible-bender', 'build', f'--builder={args.builder}']

    if args.debug:
        command.insert(1, '--debug')

    if args.extra_bender_args:
        command.extend(shlex.split(args.extra_bender_args))

    # XXX: We don't properly quote here, really with the values we are probably getting, we
    # shouldn't really have to...
    command.append(
        f'--extra-ansible-args={args.extra_ansible_args} ' +
        f'--extra-vars="{" ".join(map("=".join, extra_vars.items()))}"')
    command.append(str(image / 'playbook.yaml'))

    command.extend((base, metadata['target']))

    env = os.environ.copy()
    env['ANSIBLE_STDOUT_CALLBACK'] = 'default'

    if args.force_color or sys.stdout.isatty():
        env['ANSIBLE_FORCE_COLOR'] = 'true'

    if args.debug:
        print(' '.join(map(shlex.quote, command)))

    run(command, env=env)

    if args.export is not None:
        try:
            os.remove(args.export)
        except FileNotFoundError:
            pass

        export_image(metadata, args.export, args.builder)


if __name__ == '__main__':
    main()
