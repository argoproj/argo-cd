const pattern = /^(.*):([a-zA-Z0-9._-]*)$/;

export interface Image {
    name: string;
    newName?: string;
    newTag?: string;
    digest?: string;
}

function parseOverwrite(arg: string, overwriteImage: boolean): {name: string; digest?: string; tag?: string} {
    // match <image>@<digest>
    const parts = arg.split('@');
    if (parts.length > 1) {
        return {name: parts[0], digest: parts[1]};
    }

    // match <image>:<tag>
    const groups = pattern.exec(arg);
    if (groups && groups.length === 3) {
        return {name: groups[1], tag: groups[2]};
    }

    // match <image>
    if (arg.length > 0 && overwriteImage) {
        return {name: arg};
    }
    return {name: arg};
}

export function parse(arg: string): Image {
    // matches if there is an image name to overwrite
    // <image>=<new-image><:|@><new-tag>
    const parts = arg.split('=');
    if (parts.length === 2) {
        const overwrite = parseOverwrite(parts[1], true);
        return {
            name: parts[0],
            newName: overwrite.name,
            newTag: overwrite.tag,
            digest: overwrite.digest
        };
    }

    // matches only for <tag|digest> overwrites
    // <image><:|@><new-tag>
    const p = parseOverwrite(arg, false);
    return {name: p.name, newTag: p.tag, digest: p.digest};
}

export function format(image: Image) {
    const imageName = image.newName ? `${image.name}=${image.newName}` : image.name;
    if (image.newTag) {
        return `${imageName}:${image.newTag}`;
    } else if (image.digest) {
        return `${imageName}@${image.digest}`;
    }
    return imageName;
}
