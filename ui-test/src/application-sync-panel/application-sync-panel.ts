import {By, until, WebDriver} from 'selenium-webdriver';
import {Base} from '../base';
import Configuration from '../Configuration';
import UiTestUtilities from '../UiTestUtilities';


export class ApplicationSyncPanel extends Base {
    public static readonly BUTTON_SYNCHRONIZE: By = By.xpath('.//button[@qe-id="application-sync-panel-button-synchronize"]');
    public static readonly BUTTON_CANCEL: By = By.xpath('.//button[@qe-id="application-sync-panel-button-cancel"]');
    public constructor(driver: WebDriver) {
        super(driver);
    }

    /**
     * Click the Sync button
     */
    public async clickSyncButton() {
        try {
            // Wait until the Synchronize button appears
            const synchronizeButton = await this.driver.wait(until.elementLocated(ApplicationSyncPanel.BUTTON_SYNCHRONIZE), Configuration.TEST_TIMEOUT);
            await this.driver.wait(until.elementIsVisible(synchronizeButton), Configuration.TEST_TIMEOUT);

            // Check if the sync button is enabled
            await this.driver.wait(until.elementIsEnabled(synchronizeButton), Configuration.TEST_TIMEOUT);
            await synchronizeButton.click();

            await this.driver.wait(until.elementIsNotVisible(synchronizeButton), Configuration.TEST_SLIDING_PANEL_TIMEOUT).catch((e) => {
                UiTestUtilities.logError('The Synchronization Sliding Panel did not disappear');
                throw e;
            });
            await UiTestUtilities.captureSession(this.driver, "clickSyncButton_after.png")
            UiTestUtilities.log('Synchronize sliding panel disappeared');
        } catch (err: any) {
            throw new Error(err);
        }
    }

        /**
         * Click the Cancel Button.  Do not create the app.
         */
        public async clickCancelButton(): Promise<void> {
            try {
                const cancelButton = await UiTestUtilities.findUiElement(this.driver, ApplicationSyncPanel.BUTTON_CANCEL);
                await cancelButton.click();

                // Wait until the Create Application Sliding Panel disappears
                await this.driver.wait(until.elementIsNotVisible(cancelButton), Configuration.TEST_SLIDING_PANEL_TIMEOUT).catch((e) => {
                    UiTestUtilities.logError('The Sync Application Sliding Panel did not disappear');
                    throw e;
                });
            } catch (err: any) {
                UiTestUtilities.log('Error caught while clicking Cancel button: ' + err);
                throw new Error(err);
            }
        }
}
