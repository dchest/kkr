(function() {

  /*
   * This function adds a message to an element with
   * id #hello when the page finishes loading.
   */
  document.addEventListener("DOMContentLoaded", function (e) {
    var el = document.querySelector("#hello");
    if (el) {
      el.innerHTML = "(Also hello from JavaScript!)";
    }
  });

})();
