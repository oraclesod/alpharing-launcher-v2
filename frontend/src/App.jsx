import { Fragment, useEffect, useRef, useState } from "react";
import { EventsOn, Quit } from "../wailsjs/runtime/runtime";
import { BackgroundVideoURLs, GetInstallState, Install, LatestModVersion, OpenInstallDir, Play, RecognizeExisting, SelectSource, Uninstall } from "../wailsjs/go/main/App";
import alphaRingLogo from "./assets/logo_alpharing.png";
import bkgVideoMp4 from "./assets/bkgVideo.mp4";
import bkgVideoWebm from "./assets/bkgVideo.webm";
import "./App.css";

const CLOSE_DELAY_MS = 1000;
const ERROR_DISPLAY_MS = 6000;
const ACTION_TIMEOUT_MS = 10000;
const GAMEPAD_AXIS_DEADZONE = 0.65;
const GAMEPAD_MOVE_COOLDOWN_MS = 200;
const VIDEO_MIME_TYPES = { mp4: "video/mp4", webm: "video/webm" };
const DEFAULT_VIDEO_SOURCES = [
  { src: bkgVideoWebm, type: VIDEO_MIME_TYPES.webm },
  { src: bkgVideoMp4, type: VIDEO_MIME_TYPES.mp4 },
];

const closeWindowDelayed = () => {
  setTimeout(() => Quit(), CLOSE_DELAY_MS);
};

const SOURCES = [
  { value: "vanilla", label: "Vanilla MCC" },
  { value: "megabit", label: "MegaBit AlphaRing" },
  { value: "kirkland", label: "Kirkland (experimental)" },
];

const installedLabel = (state) => {
  switch (state.installed) {
    case "vanilla": return "Active: Vanilla (no mod DLL installed)";
    case "megabit": return `Active: MegaBit ${state.version}`;
    case "kirkland": return `Active: Kirkland ${state.version}`;
    case "unknown": return "Active: Unrecognized mod DLL";
    default: return "Active: MCC folder not located";
  }
};

function App({ buildInfo = "" }) {
  const [selectedIndex, setSelectedIndex] = useState(0);
  const selectedIndexRef = useRef(selectedIndex);
  selectedIndexRef.current = selectedIndex;
  const [showLog, setShowLog] = useState(false);
  const [log, setLog] = useState("");
  const [installState, setInstallState] = useState({ selected: "vanilla", installed: "unavailable", version: "", gameFound: false });
  const [availableVersion, setAvailableVersion] = useState("");
  const [logPosition, setLogPosition] = useState(null);
  const backgroundRef = useRef(null);
  const controlRefs = useRef([]);
  const errorRevertTimeoutRef = useRef(null);
  const forceRevertTimeoutRef = useRef(null);
  const actionRunningRef = useRef(false);
  const videoRef = useRef(null);
  const [videoSources, setVideoSources] = useState(DEFAULT_VIDEO_SOURCES);

  useEffect(() => {
    GetInstallState().then((state) => {
      setInstallState(state);
      if (state.installed === "unknown") {
        RecognizeExisting().then((recognized) => setInstallState((previous) => ({ ...recognized, selected: previous.selected })))
          .catch((error) => console.error("Could not identify installed mod:", error));
      }
    }).catch((error) => console.error("Error checking installed mod:", error));
    const refresh = () => GetInstallState().then(setInstallState).catch(console.error);
    window.addEventListener("focus", refresh);
    return () => window.removeEventListener("focus", refresh);
  }, []);

  useEffect(() => {
    if (installState.selected === "vanilla") {
      setAvailableVersion("");
      return;
    }
    let current = true;
    setAvailableVersion("");
    LatestModVersion(installState.selected)
      .then((tag) => { if (current) setAvailableVersion(tag); })
      .catch((error) => console.error("Error fetching mod version:", error));
    return () => { current = false; };
  }, [installState.selected]);

  useEffect(() => {
    BackgroundVideoURLs().then((urls) => {
      if (!urls || urls.length === 0) return;
      setVideoSources(urls.map((url) => ({ src: url, type: VIDEO_MIME_TYPES[url.split(".").pop()] })));
    }).catch((error) => console.error("Error fetching background video URLs:", error));
  }, []);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    const tryPlay = () => video.play().catch((error) => console.error("Error playing background video:", error));
    video.load();
    tryPlay();
    video.addEventListener("loadeddata", tryPlay);
    return () => video.removeEventListener("loadeddata", tryPlay);
  }, [videoSources]);

  const canLaunch = installState.selected === installState.installed ||
    (installState.selected === "vanilla" && installState.installed === "unavailable");
  const isCurrentMod = installState.selected !== "vanilla" && installState.selected === installState.installed;
  const actionLabel = installState.selected === "vanilla"
    ? (installState.installed === "vanilla" || installState.installed === "unavailable" ? "No mod installed" : "Uninstall mod")
    : (isCurrentMod ? "Uninstall" : "Install");
  const canChangeInstall = !(installState.selected === "vanilla" &&
    (installState.installed === "vanilla" || installState.installed === "unavailable"));
  const menuButtons = [
    {
      label: actionLabel,
      disabled: !canChangeInstall,
      startMessage: isCurrentMod || installState.selected === "vanilla" ? "Uninstalling mod..." : "Installing mod...",
      action: () => (isCurrentMod || installState.selected === "vanilla" ? Uninstall() : Install(installState.selected)).then(setInstallState),
      returnToMenu: true,
      longRunning: true,
    },
    { label: "Launch", disabled: !canLaunch, startMessage: "Launching MCC...", action: () => Play(installState.selected), returnToMenu: true, longRunning: true },
    { label: "Open Launcher Folder", startMessage: "Opening Folder...", action: () => OpenInstallDir(), returnToMenu: true },
    { label: "Quit to Desktop", startMessage: "Shutting Down...", action: () => closeWindowDelayed() },
  ];

  const chooseSource = (source) => {
    if (actionRunningRef.current || !SOURCES.some((item) => item.value === source)) return;
    setInstallState((previous) => ({ ...previous, selected: source }));
    SelectSource(source).then(setInstallState).catch((error) => {
      console.error("Could not save launch selection:", error);
      GetInstallState().then(setInstallState).catch(console.error);
    });
  };

  const cycleSource = (delta) => {
    const index = SOURCES.findIndex((item) => item.value === installState.selected);
    chooseSource(SOURCES[(index + delta + SOURCES.length) % SOURCES.length].value);
  };

  const moveSelection = (delta) => setSelectedIndex((selectedIndexRef.current + delta + menuButtons.length + 1) % (menuButtons.length + 1));

  const runAction = (index) => {
    if (index === 0) { cycleSource(1); return; }
    const button = menuButtons[index - 1];
    if (!button || button.disabled) return;
    if (actionRunningRef.current) return;
    actionRunningRef.current = true;
    if (errorRevertTimeoutRef.current) clearTimeout(errorRevertTimeoutRef.current);
    if (forceRevertTimeoutRef.current) clearTimeout(forceRevertTimeoutRef.current);
    setSelectedIndex(index);
    const control = controlRefs.current[index];
    const container = backgroundRef.current;
    if (control && container) {
      const buttonRect = control.getBoundingClientRect();
      const containerRect = container.getBoundingClientRect();
      setLogPosition({ top: buttonRect.top - containerRect.top, left: buttonRect.left - containerRect.left, width: buttonRect.width });
    }
    setLog(button.startMessage ?? "");
    setShowLog(true);

    // Short actions get a timeout. Play can take minutes for downloads or
    // wait for a UAC decision, so keep its progress panel active until done.
    let settled = false;
    if (!button.longRunning) {
      forceRevertTimeoutRef.current = setTimeout(() => {
        settled = true;
        actionRunningRef.current = false;
        setShowLog(false);
      }, ACTION_TIMEOUT_MS);
    }

    Promise.resolve().then(() => button.action()).then(() => {
      if (settled) return;
      actionRunningRef.current = false;
      clearTimeout(forceRevertTimeoutRef.current);
      if (button.returnToMenu) setShowLog(false);
    }).catch((error) => {
      if (settled) return;
      actionRunningRef.current = false;
      clearTimeout(forceRevertTimeoutRef.current);
      console.error("Error:", error);
      setLog(`Error: ${error}`);
      errorRevertTimeoutRef.current = setTimeout(() => setShowLog(false), ERROR_DISPLAY_MS);
    });
  };

  useEffect(() => {
    const cancel = EventsOn("log", (message) => setLog(message));
    return () => cancel?.();
  }, []);

  useEffect(() => {
    const handleKeyDown = (event) => {
      if (event.target.tagName === "SELECT") return;
      switch (event.key) {
        case "ArrowUp": case "w": case "W":
          event.preventDefault(); moveSelection(-1); break;
        case "ArrowDown": case "s": case "S":
          event.preventDefault(); moveSelection(1); break;
        case "ArrowLeft": case "a": case "A":
          event.preventDefault(); selectedIndexRef.current === 0 ? cycleSource(-1) : moveSelection(-1); break;
        case "ArrowRight": case "d": case "D":
          event.preventDefault(); selectedIndexRef.current === 0 ? cycleSource(1) : moveSelection(1); break;
        case "Enter": case " ":
          event.preventDefault(); runAction(selectedIndexRef.current); break;
        default: break;
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [installState]);

  useEffect(() => {
    let frame;
      const held = { prev: false, next: false, left: false, right: false, confirm: false };
    let lastMoveTime = 0;
    const poll = () => {
      const pads = navigator.getGamepads ? navigator.getGamepads() : [];
      const gamepad = Array.prototype.find.call(pads, Boolean);
      if (gamepad) {
        const axisX = gamepad.axes[0] ?? 0;
        const axisY = gamepad.axes[1] ?? 0;
        const prev = gamepad.buttons[12]?.pressed || axisY < -GAMEPAD_AXIS_DEADZONE;
        const next = gamepad.buttons[13]?.pressed || axisY > GAMEPAD_AXIS_DEADZONE;
        const left = gamepad.buttons[14]?.pressed || axisX < -GAMEPAD_AXIS_DEADZONE;
        const right = gamepad.buttons[15]?.pressed || axisX > GAMEPAD_AXIS_DEADZONE;
        const confirm = gamepad.buttons[0]?.pressed ?? false;
        const now = performance.now();
        // Analog stick noise near the deadzone threshold can flicker `held.prev`/`held.next`
        // false->true within a single physical push; the cooldown stops that from
        // registering as a second move.
        const canMove = now - lastMoveTime >= GAMEPAD_MOVE_COOLDOWN_MS;
        if (prev && !held.prev && canMove) { moveSelection(-1); lastMoveTime = now; }
        if (next && !held.next && canMove) { moveSelection(1); lastMoveTime = now; }
        if (left && !held.left && canMove) { selectedIndexRef.current === 0 ? cycleSource(-1) : moveSelection(-1); lastMoveTime = now; }
        if (right && !held.right && canMove) { selectedIndexRef.current === 0 ? cycleSource(1) : moveSelection(1); lastMoveTime = now; }
        if (confirm && !held.confirm) runAction(selectedIndexRef.current);
        held.prev = prev; held.next = next; held.confirm = confirm;
        held.left = left; held.right = right;
      }
      frame = requestAnimationFrame(poll);
    };
    frame = requestAnimationFrame(poll);
    return () => cancelAnimationFrame(frame);
  }, [installState]);

  return (
    <main className="container">
      <div className="background" ref={backgroundRef}>
        <video
          ref={videoRef}
          className="background-video"
          autoPlay
          loop
          muted
          playsInline
          preload="auto"
          onError={(e) => console.log("VIDEO ERROR:", e.currentTarget.error)}
          onLoadedData={() => console.log("VIDEO LOADED")}
        >
          {videoSources.map((source) => <source key={source.src} src={source.src} type={source.type} />)}
        </video>
        <img className="logo" src={alphaRingLogo} alt="AlphaRing" />
        {showLog ? <div className="log-panel" style={logPosition ?? undefined}><p className="log-message">{log || "Starting Up..."}</p></div> : (
          <div className="menu">
            <label className="source-label" htmlFor="launch-source">Launch version</label>
            <select
              id="launch-source"
              ref={(element) => { controlRefs.current[0] = element; }}
              className={`source-select${selectedIndex === 0 ? " selected" : ""}`}
              value={installState.selected}
              onFocus={() => setSelectedIndex(0)}
              onMouseEnter={() => setSelectedIndex(0)}
              onChange={(event) => chooseSource(event.target.value)}
            >
              {SOURCES.map((source) => <option key={source.value} value={source.value}>{source.label}</option>)}
            </select>
            {menuButtons.map((button, index) => <Fragment key={index}>
              <button
                ref={(element) => { controlRefs.current[index + 1] = element; }}
                type="button"
                disabled={button.disabled}
                className={`menu-button${index + 1 === selectedIndex ? " selected" : ""}`}
                onMouseEnter={() => setSelectedIndex(index + 1)}
                onClick={() => runAction(index + 1)}
              >{button.label}</button>
              {index === 1 && <div className="install-status" role="status">{installedLabel(installState)}</div>}
            </Fragment>)}
          </div>
        )}
        <p className="info">{buildInfo}{availableVersion ? ` Available: ${availableVersion}` : ""}</p>
      </div>
      <div>Fonts made from <a href="http://www.onlinewebfonts.com">Web Fonts</a> is licensed by CC BY 4.0</div>
    </main>
  );
}

export default App;
