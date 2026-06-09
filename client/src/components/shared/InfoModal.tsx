/**
 * InfoModal — tabbed app-info modal (Release Notes + Features).
 * Opened from the app (info icon) and the landing page. Features tab is a
 * placeholder until the in-app feature guide is built.
 */

import { useState } from "react";
import { useTranslation } from "react-i18next";
import Modal from "./Modal";
import ReleaseNotes from "./ReleaseNotes";
import FeatureGuide from "../help/FeatureGuide";

type InfoTab = "releaseNotes" | "features";

type InfoModalProps = {
  isOpen: boolean;
  onClose: () => void;
  initialTab?: InfoTab;
};

function InfoModal({ isOpen, onClose, initialTab = "releaseNotes" }: InfoModalProps) {
  const { t } = useTranslation("common");
  const [tab, setTab] = useState<InfoTab>(initialTab);

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="mqvi">
      <div className="info-modal">
        <div className="info-modal-tabs">
          <button
            type="button"
            className={`info-modal-tab${tab === "releaseNotes" ? " active" : ""}`}
            onClick={() => setTab("releaseNotes")}
          >
            {t("releaseNotes")}
          </button>
          <button
            type="button"
            className={`info-modal-tab${tab === "features" ? " active" : ""}`}
            onClick={() => setTab("features")}
          >
            {t("features")}
          </button>
        </div>

        <div className="info-modal-body">
          {tab === "releaseNotes" ? <ReleaseNotes /> : <FeatureGuide />}
        </div>
      </div>
    </Modal>
  );
}

export default InfoModal;
