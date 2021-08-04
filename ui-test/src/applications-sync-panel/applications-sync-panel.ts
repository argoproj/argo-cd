import {By, until, WebDriver} from 'selenium-webdriver';
import {Base} from '../base';
import * as Const from '../Constants';
import UiTestUtilities from '../UiTestUtilities';

export const SYNC_PANEL_SYNCHRONIZE_BUTTON: By = By.xpath('.//button[@qe-id="application-sync-panel-button-synchronize"]');

export class ApplicationsSyncPanel extends Base {
    public constructor(driver: WebDriver) {
        super(driver);
    }

    /**
     * Click the Sync button
     */
    public async clickSyncButton() {
        try {
            // Wait until the Synchronize button appears
            const synchronizeButton = await this.driver.wait(until.elementLocated(SYNC_PANEL_SYNCHRONIZE_BUTTON), Const.TEST_TIMEOUT);
            await this.driver.wait(until.elementIsVisible(synchronizeButton), Const.TEST_TIMEOUT);

            // Check if the sync button is enabled
            await this.driver.wait(until.elementIsEnabled(synchronizeButton), Const.TEST_TIMEOUT);
            await synchronizeButton.click();

            await this.driver.wait(until.elementIsNotVisible(synchronizeButton), Const.TEST_SLIDING_PANEL_TIMEOUT).catch(e => {
                UiTestUtilities.logError('The Synchronization Sliding Panel did not disappear');
                throw e;
            });
            UiTestUtilities.log('Synchronize sliding panel disappeared');
        } catch (err) {
            throw new Error(err);
        }
    }
}
