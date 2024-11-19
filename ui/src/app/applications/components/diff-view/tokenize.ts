import {tokenize, markEdits, MarkEditsType} from 'react-diff-view';
import {HunkData} from 'react-diff-view/types/utils';
import {HunkTokens} from 'react-diff-view/types/tokenize';

// register 'yaml' language for syntax highlighting
// import refractor from 'refractor/core';
// import yaml from 'refractor/lang/yaml';
// refractor.register(yaml);

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export default (hunks: HunkData[], editsType: MarkEditsType, oldSource: string, language: string): HunkTokens => {
    if (!hunks) {
        return undefined;
    }
    const options = {
        // NOTE: we don't use the syntax highlighting because YAML is not properly supported in refractor
        //       https://github.com/wooorm/refractor/issues/72, we will probably use monaco (as we do in the editor)
        //       to replace `react-diff-view` in the future, so we can have syntax highlighting and other features
        // highlight: language !== 'text',
        // refractor: refractor,
        // language: language,
        oldSource: oldSource,
        enhancers: [markEdits(hunks, {type: editsType})]
    };
    try {
        return tokenize(hunks, options);
    } catch (ex) {
        return undefined;
    }
};
