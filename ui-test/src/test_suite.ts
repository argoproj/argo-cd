import * as fs from 'fs';
import * as path from 'path';
import UiTestUtilities from './UiTestUtilities';
import { Navigation } from './navigation';
import Configuration from './Configuration';

async function runTestCase(filePath: string, navigation: Navigation) {
    try {
        const testModule = require(filePath);
        if (typeof testModule.doTest === 'function') {
            UiTestUtilities.log(`[RUNNING]: ${filePath}`);
            await testModule.doTest(navigation);
            UiTestUtilities.log(`[SUCCESS]: ${filePath}\n`);
        } else {
            UiTestUtilities.log(`[SKIP]: ${filePath} - No 'doTest' method exported.\n`);
        }
    } catch (error) {
        UiTestUtilities.logError(`[ERROR]: Failed to execute ${filePath}: ${error}`);
    }
}

async function runTestSuite() {
    const testCasesDir = path.join(__dirname, 'test_cases');

    const files = fs.readdirSync(testCasesDir)
        .filter(file => (file.endsWith('.ts') || file.endsWith('.js')) && !file.endsWith('.d.ts'))
        .sort();

    UiTestUtilities.log(`--- Starting Test Suite: Found ${files.length} test cases ---\n`);
    for (const file of files) {
        let navigation: Navigation = await UiTestUtilities.init();
        try {
            if (Configuration.ARGOCD_AUTH_USERNAME !== '') {
                await navigation.getLoginPage().loginWithCredentials();
            }

            await runTestCase(path.join(testCasesDir, file), navigation);
        } finally {
            UiTestUtilities.log(`[CLEANUP]: Closing browser for test ${file}`);
            await navigation.quit();
        }
    }

    UiTestUtilities.log('--- Test Suite Execution Finished ---');
}

runTestSuite().catch(err => UiTestUtilities.logError(`Suite Level Failure: ${err}`));