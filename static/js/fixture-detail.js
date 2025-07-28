document.addEventListener('DOMContentLoaded', function() {
    
    // Notes functionality
    const notesTextarea = document.getElementById('fixture-notes');
    const notesCountSpan = document.getElementById('notes-count');
    const notesStatusSpan = document.getElementById('notes-status');
    
    if (notesTextarea) {
        // Update character count
        function updateCharacterCount() {
            const currentLength = notesTextarea.value.length;
            const maxLength = 1000;
            notesCountSpan.textContent = currentLength;
            
            const counter = notesCountSpan.parentElement;
            counter.classList.remove('warning', 'error');
            
            if (currentLength > maxLength * 0.9) {
                counter.classList.add('warning');
            }
            if (currentLength >= maxLength) {
                counter.classList.add('error');
            }
        }
        
        // Listen for input events
        notesTextarea.addEventListener('input', updateCharacterCount);
        
        // Listen for HTMX events
        notesTextarea.addEventListener('htmx:afterRequest', function(evt) {
            if (evt.detail.successful) {
                notesStatusSpan.textContent = '✅ Saved';
                notesStatusSpan.className = 'notes-status success';
                setTimeout(() => {
                    notesStatusSpan.textContent = '';
                    notesStatusSpan.className = 'notes-status';
                }, 2000);
                
                // Update the glance-notes section
                updateGlanceNotes(notesTextarea.value);
                
                // Update the copyable text by regenerating it
                updateCopyableText();
            } else {
                notesStatusSpan.textContent = '❌ Error saving';
                notesStatusSpan.className = 'notes-status error';
                setTimeout(() => {
                    notesStatusSpan.textContent = '';
                    notesStatusSpan.className = 'notes-status';
                }, 3000);
            }
        });
        
        // Function to update the glance-notes section
        function updateGlanceNotes(notesText) {
            const glanceNotesDiv = document.querySelector('.glance-notes');
            const glanceContent = document.querySelector('.glance-content');
            
            if (notesText && notesText.trim()) {
                // Notes exist - show or update the glance-notes section
                if (!glanceNotesDiv) {
                    // Create the glance-notes section if it doesn't exist
                    const newGlanceNotes = document.createElement('div');
                    newGlanceNotes.className = 'glance-notes';
                    newGlanceNotes.innerHTML = `
                        <div class="glance-notes-title">📝 Notes</div>
                        <div class="glance-notes-content">${escapeHtml(notesText)}</div>
                    `;
                    // Insert before the glance-matchups section
                    const glanceMatchups = document.querySelector('.glance-matchups');
                    if (glanceMatchups) {
                        glanceMatchups.parentNode.insertAfter(newGlanceNotes, glanceMatchups);
                    } else if (glanceContent) {
                        glanceContent.appendChild(newGlanceNotes);
                    }
                } else {
                    // Update existing glance-notes content
                    const notesContent = glanceNotesDiv.querySelector('.glance-notes-content');
                    if (notesContent) {
                        notesContent.textContent = notesText;
                    }
                }
            } else {
                // No notes - remove the glance-notes section if it exists
                if (glanceNotesDiv) {
                    glanceNotesDiv.remove();
                }
            }
        }
        
        // Function to update the copyable text with current notes
        function updateCopyableText() {
            const copyableDiv = document.querySelector('.glance-copyable');
            if (!copyableDiv) return;
            
            const notesText = notesTextarea.value;
            let copyableText = copyableDiv.textContent;
            
            // Remove existing notes from copyable text
            copyableText = copyableText.replace(/📝 Notes: [^\n]*\n\n/g, '');
            
            if (notesText && notesText.trim()) {
                // Add notes after the date line
                const lines = copyableText.split('\n');
                if (lines.length >= 3) {
                    // Insert notes after the teams line (usually line 2)
                    lines.splice(3, 0, '', `📝 Notes: ${notesText}`);
                    copyableText = lines.join('\n');
                }
            }
            
            copyableDiv.textContent = copyableText;
        }
        
        // Helper function to escape HTML
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
        
        // Initialize character count
        updateCharacterCount();
    }
    
    // Add click-to-copy functionality to fixture-at-glance elements
    const copyableElements = document.querySelectorAll('.fixture-at-glance');
    
    copyableElements.forEach(element => {
        element.addEventListener('click', async function() {
            try {
                // Get the text content from the hidden copyable div
                const hiddenCopyable = this.querySelector('.glance-copyable');
                const text = hiddenCopyable ? (hiddenCopyable.textContent || hiddenCopyable.innerText) : '';
                
                // Copy to clipboard
                if (navigator.clipboard && window.isSecureContext) {
                    // Use modern clipboard API
                    await navigator.clipboard.writeText(text);
                } else {
                    // Fallback for older browsers or non-HTTPS
                    const textArea = document.createElement('textarea');
                    textArea.value = text;
                    textArea.style.position = 'fixed';
                    textArea.style.left = '-999999px';
                    textArea.style.top = '-999999px';
                    document.body.appendChild(textArea);
                    textArea.focus();
                    textArea.select();
                    document.execCommand('copy');
                    textArea.remove();
                }
                
                // Visual feedback - background color change
                this.classList.add('copied');
                
                // Create and show "Copied!" message
                const feedback = document.createElement('div');
                feedback.className = 'copy-feedback';
                feedback.textContent = '✅ Copied!';
                
                // Position feedback relative to the element
                const rect = this.getBoundingClientRect();
                feedback.style.left = (rect.left + rect.width / 2) + 'px';
                feedback.style.top = (rect.top + window.scrollY) + 'px';
                feedback.style.transform = 'translateX(-50%)';
                
                document.body.appendChild(feedback);
                
                // Animate feedback
                setTimeout(() => {
                    feedback.classList.add('show');
                }, 10);
                
                // Remove visual feedback after animation
                setTimeout(() => {
                    this.classList.remove('copied');
                    feedback.classList.remove('show');
                    setTimeout(() => {
                        if (feedback.parentNode) {
                            feedback.parentNode.removeChild(feedback);
                        }
                    }, 500);
                }, 1500);
                
            } catch (err) {
                console.error('Failed to copy text: ', err);
                
                // Show error feedback
                const errorFeedback = document.createElement('div');
                errorFeedback.className = 'copy-feedback';
                errorFeedback.style.background = '#dc3545';
                errorFeedback.textContent = '❌ Copy failed';
                
                const rect = this.getBoundingClientRect();
                errorFeedback.style.left = (rect.left + rect.width / 2) + 'px';
                errorFeedback.style.top = (rect.top + window.scrollY) + 'px';
                errorFeedback.style.transform = 'translateX(-50%)';
                
                document.body.appendChild(errorFeedback);
                
                setTimeout(() => {
                    errorFeedback.classList.add('show');
                }, 10);
                
                setTimeout(() => {
                    errorFeedback.classList.remove('show');
                    setTimeout(() => {
                        if (errorFeedback.parentNode) {
                            errorFeedback.parentNode.removeChild(errorFeedback);
                        }
                    }, 500);
                }, 2000);
            }
        });
        
        // Make it clear that the element is clickable
        element.style.cursor = 'pointer';
        element.setAttribute('title', 'Click to copy fixture information');
    });
}); 