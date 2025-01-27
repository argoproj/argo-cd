import Configuration from './Configuration';
import {Builder, By, until, WebDriver, WebElement} from 'selenium-webdriver';
import chrome from 'selenium-webdriver/chrome';
import * as Const from './Constants';
import {Navigation} from './navigation';

export default class UiTestUtilities {
    /**
     * Log a message to the console.
     * @param message
     */
    public static async log(message: string): Promise<void> {
        let doLog = Const.ENABLE_CONSOLE_LOG;
        // Config override
        if (Configuration.ENABLE_CONSOLE_LOG) {
            if (Configuration.ENABLE_CONSOLE_LOG === 'false') {
                doLog = false;
            } else {
                doLog = true;
            }
        }
        if (doLog) {
            // tslint:disable-next-line:no-console
            console.log(message);
        }
    }

    public static async logError(message: string): Promise<void> {
        let doLog = Const.ENABLE_CONSOLE_LOG;
        // Config override
        if (Configuration.ENABLE_CONSOLE_LOG) {
            if (Configuration.ENABLE_CONSOLE_LOG === 'false') {
                doLog = false;
            } else {
                doLog = true;
            }
        }
        if (doLog) {
            // tslint:disable-next-line:no-console
            console.error(message);
        }
    }

    /**
     * Set up the WebDriver. Initial steps for all tests.  Returns the instance of Navigation with the WebDriver.
     * From there, navigate the UI.  Test cases do no need to reference the instance of WebDriver since Component/Page-specific
     * API methods should be called instead.
     *
     */
    public static async init(): Promise<Navigation> {
        const options = new chrome.Options();
        if (process.env.IS_HEADLESS) {
            options.addArguments('headless');
        }
        options.addArguments('window-size=1400x1200');
        const driver = await new Builder()
            .forBrowser('chrome')
            .setChromeOptions(options)
            .build();

        UiTestUtilities.log('Environment variables are:');
        UiTestUtilities.log(require('dotenv').config({path: __dirname + '/../.env'}));

        // Navigate to the ArgoCD URL
        await driver.get(Configuration.ARGOCD_URL);

        return new Navigation(driver);
    }

    /**
     * Locate the UI Element for the given locator, and wait until it is visible
     *
     * @param driver
     * @param locator
     */
    public static async findUiElement(driver: WebDriver, locator: By): Promise<WebElement> {
        try {
            let timeout = Const.TEST_TIMEOUT;
            if (Configuration.TEST_TIMEOUT) {
                timeout = parseInt(Configuration.TEST_TIMEOUT, 10);
            }
            const element = await driver.wait(until.elementLocated(locator), timeout);
            var isDisplayed = await element.isDisplayed();
            if (isDisplayed) {
                await driver.wait(until.elementIsVisible(element), timeout);
            }
            return element;
        } catch (err) {
            throw err;
        }
    }

    /**
     * Similar to until.methods and used in driver.wait, this will wait until
     * the expected attribute is the same as the actual attribute on the element
     *
     * @param attr
     * @param attrValue
     */
    public static async untilAttributeIs(element: WebElement, attr: string, attrValue: string): Promise<boolean> {
        const actual = await element.getAttribute(attr);
        UiTestUtilities.log('Actual = ' + actual + ', expected = ' + attrValue + ', ' + (actual === attrValue));
        return actual === attrValue;
    }

    /**
     * Similar to until.methods and used in driver.wait, this function will wait until
     * the element (eg. operation state) title attribute no longer is present
     *
     * @param element
     */
    public static async untilOperationStatusDisappears(element: WebElement): Promise<boolean> {
        try {
            const opState = await element.getAttribute('title');
            UiTestUtilities.log('Operation State = ' + opState);
            return false;
        } catch (err) {
            UiTestUtilities.log('Status disappeared');
            return true;
        }
    }

    /**
     * For clicking on elements if WebElement.click() doesn't work
     *
     * @param driver
     * @param element
     */
    public static async click(driver: WebDriver, element: WebElement): Promise<void> {
        try {
            // Execute synchronous script
            await driver.executeScript('arguments[0].click();', element);
        } catch (e) {
            throw e;
        }
    }
}
