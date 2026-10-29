import React from 'react';
import ClusterSwitcher from './ClusterSwitcher';

interface HeaderProps {
  activeMenu: string;
  setActiveMenu: (menu: string) => void;
  onManualRefresh: () => void;
}

const Header: React.FC<HeaderProps> = ({
  activeMenu,
  setActiveMenu,
  onManualRefresh
}) => {
  return (
    <header className="header">
      <div className="service-name">K-Watch</div>
      
      <nav className="navigation">
        <button
          type="button"
          className={`nav-item ${activeMenu === 'overview' ? 'active' : ''}`}
          onClick={() => setActiveMenu('overview')}
        >
          Overview
        </button>
        <button
          type="button"
          className={`nav-item ${activeMenu === 'resources' ? 'active' : ''}`}
          onClick={() => setActiveMenu('resources')}
        >
          Map
        </button>
      </nav>

      <div className="action-controls">
        <ClusterSwitcher />
        {(activeMenu === 'overview' || activeMenu === 'resources') && (
          <button className="refresh-button" onClick={onManualRefresh}>
            <svg fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
        )}
        

      </div>
    </header>
  );
};

export default Header;
